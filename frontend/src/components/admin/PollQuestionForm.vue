<template>
  <form class="pq-form stack" @submit.prevent="submit">
    <div class="stack" style="gap: 4px;">
      <label :for="`pq-text-${uid}`" class="bold">Question</label>
      <input
        :id="`pq-text-${uid}`"
        ref="textInput"
        v-model="text"
        placeholder="e.g. Name something you always forget to pack."
        maxlength="300"
      />
    </div>

    <div class="stack" style="gap: 4px;">
      <div class="row between" style="align-items: baseline;">
        <span class="bold">Top 5 answers</span>
        <span class="muted" style="font-size: .8rem;">points = how many people said it</span>
      </div>
      <div v-for="(a, i) in answers" :key="i" class="pq-answer">
        <span class="pq-answer__rank" aria-hidden="true">{{ i + 1 }}</span>
        <input
          v-model="a.text"
          :aria-label="`Answer ${i + 1} text`"
          :placeholder="answerPlaceholders[i]"
          maxlength="120"
        />
        <input
          v-model.number="a.points"
          type="number"
          min="0"
          step="1"
          :aria-label="`Answer ${i + 1} points`"
          class="pq-answer__pts"
        />
      </div>
      <p class="muted" style="margin: 0; font-size: .8rem;">
        List them most-popular first — they're shuffled before the teams see them.
      </p>
    </div>

    <div v-if="shownProblem" class="error">{{ shownProblem }}</div>
    <div v-if="saveError" class="error">{{ saveError }}</div>

    <div class="row wrap" style="gap: 8px;">
      <button type="submit" class="btn-primary" :disabled="busy">
        {{ busy ? 'Saving…' : (mode === 'edit' ? 'Save changes' : 'Add question') }}
      </button>
      <button type="button" class="btn-ghost" :disabled="busy" @click="emit('cancel')">Cancel</button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { adminApi } from '@/services/api'
import { errMsg } from '@/composables/errMsg'
import type { ImportAnswer } from '@/services/api'
import type { PollOption, Question } from '@/types'

const props = defineProps<{ code: string; question?: Question | null }>()
const emit = defineEmits<{ (e: 'saved'): void; (e: 'cancel'): void }>()

const ANSWER_COUNT = 5
const answerPlaceholders = ['Most popular answer', 'Second', 'Third', 'Fourth', 'Fifth']

const uid = Math.random().toString(36).slice(2, 8)
const mode = computed(() => (props.question ? 'edit' : 'add'))

const textInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)
const saveError = ref('')
// A blank form is not a mistake the host has made yet, so say nothing until
// they try to submit. After that the message tracks the fields live.
const attempted = ref(false)

function blankAnswers(): ImportAnswer[] {
  return Array.from({ length: ANSWER_COUNT }, () => ({ text: '', points: 0 }))
}

// An existing question is stored with its options shuffled, so rank them by
// points for editing — that's the order the survey results arrive in and the
// order a human thinks about them.
function answersFrom(q: Question): ImportAnswer[] {
  const opts = [...((q.options || []) as PollOption[])]
    .map(o => ({ text: o.text, points: o.points ?? 0 }))
    .sort((a, b) => b.points - a.points)
  while (opts.length < ANSWER_COUNT) opts.push({ text: '', points: 0 })
  return opts.slice(0, ANSWER_COUNT)
}

const text = ref(props.question?.text || '')
const answers = ref<ImportAnswer[]>(props.question ? answersFrom(props.question) : blankAnswers())

// Mirrors the server's rules so a rejected submit is explained here rather than
// by the API. The backend still re-validates — this is guidance, not the guard.
const problem = computed(() => {
  if (!text.value.trim()) return 'The question needs some text.'
  const seen = new Set<string>()
  for (let i = 0; i < answers.value.length; i++) {
    const a = answers.value[i]
    const t = a.text.trim()
    if (!t) return `Answer ${i + 1} is empty — all ${ANSWER_COUNT} are needed.`
    const key = t.toLowerCase()
    if (seen.has(key)) return `"${t}" is listed twice.`
    seen.add(key)
    if (typeof a.points !== 'number' || !isFinite(a.points) || a.points < 0) {
      return `Answer ${i + 1} needs a points value of 0 or more.`
    }
  }
  return ''
})

const shownProblem = computed(() => (attempted.value ? problem.value : ''))

onMounted(async () => {
  await nextTick()
  textInput.value?.focus()
})

async function submit() {
  attempted.value = true
  if (problem.value) return
  saveError.value = ''
  busy.value = true
  const body = {
    text: text.value.trim(),
    answers: answers.value.map(a => ({ text: a.text.trim(), points: Number(a.points) })),
  }
  try {
    if (props.question) {
      await adminApi.updateQuestion(props.code, props.question.id, body)
    } else {
      await adminApi.createQuestion(props.code, body)
      // Adding is usually a run of several, so reset and stay put rather than
      // making the host reopen the form for each one.
      text.value = ''
      answers.value = blankAnswers()
      // The fresh blank form is not an error either.
      attempted.value = false
      textInput.value?.focus()
    }
    emit('saved')
  } catch (e) {
    saveError.value = errMsg(e, 'Could not save')
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.pq-form {
  border: 3px solid var(--ink);
  background: var(--paper);
  padding: 14px;
}
.pq-answer {
  display: grid;
  grid-template-columns: auto 1fr 90px;
  gap: 8px;
  align-items: center;
}
.pq-answer__rank {
  width: 1.6em;
  text-align: center;
  font-weight: 900;
  opacity: .5;
}
.pq-answer__pts { text-align: right; }
</style>
