<template>
  <main class="stack-lg">
    <transition name="fade" mode="out-in">
      <div v-if="saved && !editing" key="waiting" class="card card--mint stack center card-stickered">
        <div style="font-size: 3rem; line-height: 1;">🎉</div>
        <h1>Locked in!</h1>
        <p>Your question is ready. Sit tight — the host kicks things off in a moment.</p>
        <button class="btn-ghost" @click="startEdit">← Edit my question</button>
      </div>

      <!-- Step 1: Photo -->
      <div v-else-if="step === 'photo'" key="photo" class="card stack">
        <Stepper :current="1" :photo="photo" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 1 of 3</span>
        <h1 style="margin: 16px 0 0;">Set up your question for the quiz</h1>
        <p class="muted" style="margin-top: 16px;">
          Start with a photo of whatever your question is about.
        </p>

        <PhotoPicker v-model="photo" />

        <button class="btn-primary btn-lg btn-block" :disabled="!photo" @click="step = 'ai-choice'">
          Continue →
        </button>
        <button v-if="saved" class="btn-link" @click="cancelEdit">Cancel</button>
      </div>

      <!-- Step 2: AI or manual -->
      <div v-else-if="step === 'ai-choice'" key="ai-choice" class="card stack">
        <Stepper :current="2" :photo="photo" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 2 of 3</span>
        <h1 style="margin: 16px 0 0;">How should we make it?</h1>
        <p class="muted" style="margin-top: 16px;">
          Let AI suggest a question for your photo, or write your own.
        </p>

        <div class="photo-strip">
          <img :src="photo" alt="" />
        </div>

        <button class="path-card path-card--ai" @click="useAIPath">
          <div class="path-card__icon" aria-hidden="true">✨</div>
          <div class="path-card__body">
            <div class="path-card__title">Help me with AI</div>
            <div class="path-card__desc">Generate a question and answer options from my photo.</div>
          </div>
          <div class="path-card__chev" aria-hidden="true">→</div>
        </button>
        <button class="path-card" @click="useManualPath">
          <div class="path-card__icon" aria-hidden="true">✍️</div>
          <div class="path-card__body">
            <div class="path-card__title">I'll write it myself</div>
            <div class="path-card__desc">Type the question and pick the answer manually.</div>
          </div>
          <div class="path-card__chev" aria-hidden="true">→</div>
        </button>

        <div v-if="err" class="error">{{ err }}</div>
        <button class="btn-link" @click="step = 'photo'">← Back to photo</button>
      </div>

      <!-- Step 3: Editor -->
      <div v-else key="editor" class="card stack">
        <Stepper :current="3" :photo="photo" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 3 of 3</span>
        <h1 style="margin: 16px 0 0;">Your question</h1>
        <p class="muted" style="margin-top: 16px;">Write the question and set the right answer.</p>

        <div class="photo-summary">
          <img class="photo-thumb" :src="photo" alt="" />
          <div class="photo-summary__meta">
            <div class="photo-summary__label">Photo</div>
            <button class="btn-link" @click="step = 'photo'">Change photo</button>
          </div>
        </div>

        <label>Question</label>
        <textarea v-model="text" placeholder="What is this thing?" maxlength="160" rows="3"></textarea>

        <label>Answer type</label>
        <div class="toggles">
          <button :class="{ active: answerType === 'yesno' }" @click="answerType = 'yesno'">Yes / No</button>
          <button :class="{ active: answerType === 'choice' }" @click="answerType = 'choice'">Multiple</button>
          <button :class="{ active: answerType === 'number' }" @click="answerType = 'number'">Number</button>
        </div>

        <template v-if="answerType === 'yesno'">
          <label>Correct answer</label>
          <div class="grid-2">
            <button :class="['option-btn', correct === 'yes' && 'chosen']" @click="correct = 'yes'">
              <span class="option-btn__bullet">Y</span>Yes
            </button>
            <button :class="['option-btn', correct === 'no' && 'chosen']" @click="correct = 'no'">
              <span class="option-btn__bullet">N</span>No
            </button>
          </div>
        </template>

        <template v-if="answerType === 'choice'">
          <label>Options · tap ★ to mark the right one</label>
          <div class="stack">
            <div v-for="(_, i) in options" :key="i" class="row" style="align-items: stretch;">
              <input
                v-model="options[i]"
                :placeholder="`Option ${i + 1}`"
                maxlength="60"
              />
              <button
                :class="['btn-icon', correctIdx === i ? 'btn-warn' : 'btn-ghost']"
                @click="correctIdx = i"
                :aria-label="`Mark option ${i + 1} as correct`"
                :title="correctIdx === i ? 'Correct answer' : 'Mark as correct'"
              >★</button>
              <button
                v-if="options.length > 2"
                class="btn-icon btn-danger"
                @click="removeOption(i)"
                aria-label="Remove option"
              >✕</button>
            </div>
            <button v-if="options.length < 4" class="btn-ghost btn-block" @click="options.push('')">+ Add option</button>
          </div>
        </template>

        <template v-if="answerType === 'number'">
          <label>Correct number</label>
          <input v-model.number="correctNumber" type="number" step="any" placeholder="42" />
        </template>

        <div v-if="!aiConfirm" class="row">
          <button class="btn-blue btn-block" @click="requestAI" :disabled="aiBusy">
            <span aria-hidden="true">✨</span>
            {{ aiBusy ? 'Thinking…' : 'Help me with AI' }}
          </button>
        </div>
        <div v-else class="card card--blue stack" style="padding: 14px;">
          <p style="margin: 0;">AI will replace your current question{{ extraFieldsLabel }}. Continue?</p>
          <div class="row">
            <button class="btn-primary flex-1" @click="confirmAI">Replace</button>
            <button class="btn-ghost flex-1" @click="aiConfirm = false">Keep mine</button>
          </div>
        </div>

        <div class="row">
          <button class="btn-primary btn-lg flex-1" :disabled="!canSubmit || loading" @click="save">
            {{ loading ? 'Saving…' : (saved ? 'Update question' : 'Save question') }}
          </button>
          <button v-if="saved" class="btn-ghost" @click="cancelEdit">Cancel</button>
        </div>
        <div v-if="err" class="error">{{ err }}</div>
      </div>
    </transition>

    <transition name="fade">
      <div v-if="aiBusy" class="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="ai-busy-title">
        <div class="modal stack center">
          <div class="spinner" aria-hidden="true"></div>
          <h2 id="ai-busy-title" style="margin: 0;">Cooking up a question…</h2>
          <p class="muted" style="margin: 0;">Usually takes a few seconds.</p>
        </div>
      </div>
    </transition>

    <div class="card card--cream">
      <div class="row between" style="margin-bottom: 12px;">
        <h2 style="margin: 0;">Players in the room</h2>
        <span class="tag tag--blue">{{ users.length }}</span>
      </div>
      <div class="row wrap" style="gap: 10px;">
        <div v-for="u in users" :key="u.id" class="who" style="box-shadow: 2px 2px 0 var(--ink);">
          <img class="avatar avatar-sm" :src="u.photoB64 || ''" :alt="u.name" />
          <div class="who__meta"><span class="who__name">{{ u.name }}</span></div>
        </div>
        <span v-if="!users.length" class="muted">No one yet — be the first!</span>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, h } from 'vue'
import { useRouter } from 'vue-router'
import PhotoPicker from '../components/PhotoPicker.vue'
import { api } from '../services/api'
import { useGameStore } from '../stores/game'
import type { AnswerType, Question } from '../types'

const props = defineProps<{ code: string }>()
const router = useRouter()
const store = useGameStore()

const photo = ref('')
const text = ref('')
const answerType = ref<AnswerType>('yesno')
const correct = ref<'yes' | 'no'>('yes')
const options = ref<string[]>(['', ''])
const correctIdx = ref(0)
const correctNumber = ref<number | ''>(0)
const loading = ref(false)
const saved = ref(false)
const editing = ref(false)
const err = ref('')
const aiBusy = ref(false)
const aiConfirm = ref(false)
const step = ref<'photo' | 'ai-choice' | 'editor'>('photo')

interface StepperProps { current: number; photo: string }
const Stepper = {
  props: {
    current: { type: Number, required: true },
    photo: { type: String, required: true },
  },
  setup(p: StepperProps) {
    return () => {
      const steps = [
        { n: 1, label: 'Photo' },
        { n: 2, label: 'AI?' },
        { n: 3, label: 'Details' },
      ]
      const nodes: ReturnType<typeof h>[] = []
      steps.forEach((s, i) => {
        const state = s.n < p.current ? 'done' : s.n === p.current ? 'active' : ''
        nodes.push(h('div', { class: ['stepper__step', state] }, [
          h('div', { class: 'stepper__dot' }, state === 'done' ? '✓' : String(s.n)),
          h('div', { class: 'stepper__label' }, s.label),
        ]))
        if (i < steps.length - 1) {
          nodes.push(h('div', { class: ['stepper__bar', s.n < p.current ? 'done' : ''] }))
        }
      })
      return h('div', { class: 'stepper', 'aria-label': `Step ${p.current} of 3` }, nodes)
    }
  },
}

const users = computed(() => store.users)

const hasContent = computed(() => {
  if (text.value.trim()) return true
  if (answerType.value === 'choice' && options.value.some(o => o.trim())) return true
  if (answerType.value === 'number' && correctNumber.value) return true
  return false
})

const extraFieldsLabel = computed(() => {
  if (answerType.value === 'choice' && options.value.some(o => o.trim())) return ' and options'
  if (answerType.value === 'number' && correctNumber.value) return ' and number'
  return ''
})

const canSubmit = computed(() => {
  if (!photo.value || !text.value.trim()) return false
  if (answerType.value === 'choice') {
    const filled = options.value.map(o => o.trim()).filter(Boolean)
    return filled.length >= 2 && correctIdx.value < options.value.length
  }
  if (answerType.value === 'number') {
    return typeof correctNumber.value === 'number' && !Number.isNaN(correctNumber.value)
  }
  return true
})

onMounted(async () => {
  await store.loadMe()
  if (!store.me) { router.replace('/'); return }
  store.ensureWS()
  try {
    const list = await api.listUsers(props.code)
    store.users = list
  } catch {}
  if (store.game && store.game.state === 'game') router.replace(`/g/${props.code}/play`)
  if (store.game && store.game.state === 'finished') router.replace(`/g/${props.code}/results`)

  try {
    const qs = await api.listQuestions(props.code)
    const mine = qs.find(q => q.userId === store.me?.id)
    if (mine) hydrateFromQuestion(mine)
  } catch {}
})

watch(() => store.game && store.game.state, (s) => {
  if (s === 'game') router.replace(`/g/${props.code}/play`)
  if (s === 'finished') router.replace(`/g/${props.code}/results`)
})

function hydrateFromQuestion(q: Question) {
  text.value = q.text
  photo.value = q.photoB64
  answerType.value = q.answerType
  if (q.answerType === 'choice') {
    options.value = Array.isArray(q.options) && q.options.length ? q.options : ['', '']
  }
  saved.value = true
}

function removeOption(i: number) {
  options.value.splice(i, 1)
  if (correctIdx.value >= options.value.length) correctIdx.value = 0
}

function startEdit() {
  editing.value = true
  step.value = 'editor'
}

function cancelEdit() {
  editing.value = false
  err.value = ''
}

function useManualPath() {
  err.value = ''
  step.value = 'editor'
}

async function useAIPath() {
  err.value = ''
  aiBusy.value = true
  try {
    const r = await api.aiSuggest({
      hint: '',
      answerType: 'choice',
      photoB64: photo.value,
    })
    answerType.value = 'choice'
    text.value = r.text || ''
    if (Array.isArray(r.options) && r.options.length >= 2) {
      options.value = r.options.slice(0, 4)
      correctIdx.value = Number(r.correct) || 0
    }
    step.value = 'editor'
  } catch (e) {
    err.value = 'AI: ' + ((e as Error).message || 'failed')
  } finally {
    aiBusy.value = false
  }
}

async function save() {
  err.value = ''
  loading.value = true
  try {
    const body: {
      text: string
      photoB64: string
      answerType: AnswerType
      options: string[]
      correct?: string | number
    } = {
      text: text.value.trim(),
      photoB64: photo.value,
      answerType: answerType.value,
      options: answerType.value === 'choice' ? options.value.map(o => o.trim()).filter(Boolean) : [],
      correct: undefined,
    }
    if (answerType.value === 'yesno') body.correct = correct.value
    else if (answerType.value === 'choice') body.correct = correctIdx.value
    else body.correct = Number(correctNumber.value)
    await api.putQuestion(props.code, body)
    saved.value = true
    editing.value = false
  } catch (e) {
    err.value = (e as Error).message || 'Could not save'
  } finally {
    loading.value = false
  }
}

function requestAI() {
  if (hasContent.value) { aiConfirm.value = true; return }
  confirmAI()
}

async function confirmAI() {
  aiConfirm.value = false
  err.value = ''
  aiBusy.value = true
  try {
    const r = await api.aiSuggest({
      hint: text.value || '',
      answerType: 'choice',
      photoB64: photo.value,
    })
    answerType.value = 'choice'
    text.value = r.text || text.value
    if (Array.isArray(r.options) && r.options.length >= 2) {
      options.value = r.options.slice(0, 4)
      correctIdx.value = Number(r.correct) || 0
    }
  } catch (e) {
    err.value = 'AI: ' + ((e as Error).message || 'failed')
  } finally {
    aiBusy.value = false
  }
}
</script>
