import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const createQuestion = vi.fn()
const updateQuestion = vi.fn()

vi.mock('@/services/api', () => ({
  adminApi: {
    createQuestion: (...a: unknown[]) => createQuestion(...a),
    updateQuestion: (...a: unknown[]) => updateQuestion(...a),
  },
}))

import PollQuestionForm from './PollQuestionForm.vue'
import type { Question } from '@/types'

const fiveGood = [
  { text: 'Toothbrush', points: 41 },
  { text: 'Charger', points: 22 },
  { text: 'Socks', points: 11 },
  { text: 'Sunscreen', points: 7 },
  { text: 'Passport', points: 4 },
]

function fill(w: ReturnType<typeof mount>, text: string, answers: { text: string; points: number }[]) {
  const inputs = w.findAll('input')
  // [0] is the question text, then text/points pairs per answer.
  return (async () => {
    await inputs[0].setValue(text)
    for (let i = 0; i < answers.length; i++) {
      await inputs[1 + i * 2].setValue(answers[i].text)
      await inputs[2 + i * 2].setValue(String(answers[i].points))
    }
  })()
}

function problemText(w: ReturnType<typeof mount>): string {
  const e = w.find('.error')
  return e.exists() ? e.text() : ''
}

describe('PollQuestionForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('says nothing about an untouched blank form', async () => {
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })
    expect(problemText(w)).toBe('')
    expect(w.find('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('blocks submission until the question has text and five answers', async () => {
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })

    await w.find('form').trigger('submit')
    expect(createQuestion).not.toHaveBeenCalled()
    expect(problemText(w)).toContain('needs some text')

    await fill(w, 'Name something you always forget to pack.', fiveGood.slice(0, 4).concat([{ text: '', points: 0 }]))
    expect(problemText(w)).toContain('Answer 5 is empty')

    await fill(w, 'Name something you always forget to pack.', fiveGood)
    expect(problemText(w)).toBe('')
  })

  it('rejects a duplicate answer regardless of case', async () => {
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })
    await fill(w, 'Q', [
      { text: 'Pizza', points: 40 },
      { text: 'PIZZA', points: 20 },
      ...fiveGood.slice(2),
    ])
    await w.find('form').trigger('submit')
    expect(createQuestion).not.toHaveBeenCalled()
    expect(problemText(w)).toContain('listed twice')
  })

  it('rejects negative points', async () => {
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })
    await fill(w, 'Q', [{ text: 'A', points: -1 }, ...fiveGood.slice(1)])
    await w.find('form').trigger('submit')
    expect(createQuestion).not.toHaveBeenCalled()
    expect(problemText(w)).toContain('points value of 0 or more')
  })

  it('creates a question and clears the form so the next one can be typed', async () => {
    createQuestion.mockResolvedValue({})
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })
    await fill(w, '  Name a bad excuse.  ', fiveGood)
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(createQuestion).toHaveBeenCalledTimes(1)
    const [code, body] = createQuestion.mock.calls[0]
    expect(code).toBe('consensus')
    expect(body.text).toBe('Name a bad excuse.')
    expect(body.answers).toEqual(fiveGood)

    // Adding is usually a run of several, so the form resets in place — and the
    // blank form it resets to must not accuse the host of an empty question.
    expect((w.findAll('input')[0].element as HTMLInputElement).value).toBe('')
    expect(problemText(w)).toBe('')
    expect(w.emitted('saved')).toHaveLength(1)
  })

  it('editing loads the stored options ranked by points, not in stored order', async () => {
    const question: Question = {
      id: 'q1',
      userId: '',
      text: 'Existing',
      answerType: 'poll',
      correct: '',
      // Stored shuffled, exactly as the backend writes them.
      options: [
        { text: 'Socks', points: 11 },
        { text: 'Toothbrush', points: 41 },
        { text: 'Passport', points: 4 },
        { text: 'Charger', points: 22 },
        { text: 'Sunscreen', points: 7 },
      ],
    }
    const w = mount(PollQuestionForm, { props: { code: 'consensus', question } })
    const inputs = w.findAll('input')
    const shown = [0, 1, 2, 3, 4].map(i => (inputs[1 + i * 2].element as HTMLInputElement).value)
    expect(shown).toEqual(['Toothbrush', 'Charger', 'Socks', 'Sunscreen', 'Passport'])
  })

  it('editing calls update with the question id and does not clear the form', async () => {
    updateQuestion.mockResolvedValue({})
    const question: Question = {
      id: 'q1', userId: '', text: 'Existing', answerType: 'poll', correct: '',
      options: fiveGood,
    }
    const w = mount(PollQuestionForm, { props: { code: 'consensus', question } })
    await w.findAll('input')[0].setValue('Edited text')
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(updateQuestion).toHaveBeenCalledTimes(1)
    const [code, id, body] = updateQuestion.mock.calls[0]
    expect([code, id]).toEqual(['consensus', 'q1'])
    expect(body.text).toBe('Edited text')
    expect(createQuestion).not.toHaveBeenCalled()
    expect((w.findAll('input')[0].element as HTMLInputElement).value).toBe('Edited text')
  })

  it('surfaces a server error instead of silently swallowing it', async () => {
    createQuestion.mockRejectedValue(new Error('duplicate answer "Pizza"'))
    const w = mount(PollQuestionForm, { props: { code: 'consensus' } })
    await fill(w, 'Q', fiveGood)
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('duplicate answer')
  })
})
