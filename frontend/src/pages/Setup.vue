<template>
  <main class="stack-lg">
    <div v-if="!initialReady" class="card card--cream stack center" aria-busy="true">
      <div class="spinner" aria-hidden="true"></div>
    </div>

    <transition v-else name="fade" mode="out-in">
      <div v-if="saved && !editing" key="waiting" class="card card--mint stack center card-stickered">
        <div style="font-size: 3rem; line-height: 1;">🎉</div>
        <h1>Locked in!</h1>
        <p>Your question is ready. Sit tight — the host kicks things off in a moment.</p>

        <div v-if="offerEmail" class="email-offer stack">
          <p class="email-offer__lead">
            <strong>Got a minute?</strong> The game won't start for a while. Drop us your email
            and we'll send a one-click link so you can rejoin from any device.
          </p>
          <label for="locked-email" class="sr-only">Email</label>
          <div class="row" style="gap: 8px;">
            <input
              id="locked-email"
              v-model="lockedEmail"
              type="email"
              placeholder="you@example.com"
              maxlength="120"
              autocomplete="email"
              inputmode="email"
              style="flex: 1;"
            />
            <button
              class="btn-primary"
              :disabled="!lockedEmailValid || savingEmail"
              @click="saveLockedEmail"
            >
              {{ savingEmail ? '…' : 'Send link' }}
            </button>
          </div>
          <div v-if="emailErr" class="error">{{ emailErr }}</div>
          <div v-if="emailSent" class="email-offer__sent">Link sent! Check your inbox.</div>
        </div>

        <button class="btn-ghost" @click="startEdit">← Edit my question</button>
      </div>

      <!-- Step 1: Photo -->
      <div v-else-if="step === 'photo'" key="photo" class="card stack">
        <Stepper :current="1" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 1 of 3</span>
        <h1 style="margin: 16px 0 0;">Set up your question for the quiz</h1>
        <p class="muted" style="margin-top: 16px;">
          Start with a photo of whatever your question is about.
        </p>
        <p v-if="showWaitNotice" class="wait-notice">
          If you wait here, you will still participate in the game when it starts.
        </p>

        <PhotoPicker v-model:image-id="photoId" @busy="pickerBusy = $event" />

        <button class="btn-primary btn-lg btn-block" :disabled="!photoId || pickerBusy" @click="step = 'ai-choice'">
          Continue →
        </button>
        <button v-if="saved" class="btn-link" @click="cancelEdit">Cancel</button>
      </div>

      <!-- Step 2: AI or manual -->
      <div v-else-if="step === 'ai-choice'" key="ai-choice" class="card stack">
        <Stepper :current="2" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 2 of 3</span>
        <h1 style="margin: 16px 0 0;">How should we make it?</h1>
        <p class="muted" style="margin-top: 16px;">
          Let AI suggest a question for your photo, or write your own.
        </p>

        <div class="photo-strip">
          <img :src="imageUrl(photoId, 'medium')" alt="" loading="lazy" decoding="async" />
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
        <Stepper :current="3" />
        <span class="tag tag--yellow" style="align-self: flex-start;">Step 3 of 3</span>
        <h1 style="margin: 16px 0 0;">Your question</h1>
        <p class="muted" style="margin-top: 16px;">Write the question and set the right answer.</p>

        <div class="photo-summary">
          <img class="photo-thumb" :src="imageUrl(photoId, 'thumb')" alt="" loading="lazy" decoding="async" />
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
          <button ref="saveBtnRef" class="btn-primary btn-lg flex-1" :disabled="!canSubmit || loading" @click="save">
            {{ loading ? 'Saving…' : (saved ? 'Update question' : 'Save question') }}
          </button>
          <button v-if="saved" class="btn-ghost" @click="cancelEdit">Cancel</button>
        </div>
        <div v-if="err" class="error">{{ err }}</div>
      </div>
    </transition>

    <transition name="fade">
      <button v-if="showSaveHint" class="save-hint" type="button" @click="scrollToSave">
        <span>Scroll down to save your question</span>
        <span class="save-hint__arrow" aria-hidden="true">↓</span>
      </button>
    </transition>

    <transition name="fade">
      <div v-if="aiBusy" class="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="ai-busy-title">
        <div class="modal stack center">
          <div class="spinner" aria-hidden="true"></div>
          <h2 id="ai-busy-title" style="margin: 0;">Cooking up a question…</h2>
          <p class="muted" style="margin: 10px 0 0 0;">Lowkey just wait 90 seconds, our AI Overlords are working on it.</p>
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
          <img class="avatar avatar-sm" :src="imageUrl(u.photoImageId, 'thumb')" :alt="u.name" loading="lazy" decoding="async" />
          <div class="who__meta"><span class="who__name">{{ u.name }}</span></div>
        </div>
        <span v-if="!users.length" class="muted">No one yet — be the first!</span>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import PhotoPicker from '@/components/PhotoPicker.vue'
import Stepper from '@/components/Stepper.vue'
import { playerApi } from '@/services/api'
import { imageUrl } from '@/services/images'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'
import { useSaveHint } from '@/composables/useSaveHint'
import type { AnswerType, Question } from '@/types'

const props = defineProps<{ code: string }>()
const router = useRouter()
const store = useGameStore()

const photoId = ref('')
const pickerBusy = ref(false)
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
const initialReady = ref(false)
const nowMs = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | null = null
const saveBtnRef = ref<HTMLButtonElement | null>(null)
const { visible: showSaveHint, arm: armSaveHint, dismiss: dismissSaveHint, scrollTo: scrollToSave } = useSaveHint(saveBtnRef)

const lockedEmail = ref('')
const savingEmail = ref(false)
const emailSent = ref(false)
const emailErr = ref('')

const users = computed(() => store.users)

const WAIT_NOTICE_THRESHOLD_MS = 60 * 60 * 1000

// Without a scheduled start, the host kicks off at will so the wait notice
// always applies. With one, only show it once we're within the threshold so
// players don't sit on the page for hours.
const withinThreshold = computed(() => {
  const sched = store.game?.scheduledAt
  if (!sched) return true
  const startMs = new Date(sched).getTime()
  if (isNaN(startMs)) return true
  const serverNow = nowMs.value + store.serverClockOffsetMs
  return startMs - serverNow <= WAIT_NOTICE_THRESHOLD_MS
})
const showWaitNotice = withinThreshold

// Email pitch only makes sense outside the threshold (player is going to walk
// away) and only if they haven't already given one.
const offerEmail = computed(() => !withinThreshold.value && !(store.me?.email || '').trim() && !emailSent.value)
const lockedEmailValid = computed(() => /.+@.+\..+/.test(lockedEmail.value.trim()))

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
  if (!photoId.value || !text.value.trim()) return false
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
    store.setUsers(await playerApi.listUsers(props.code))
  } catch {}
  if (store.game && store.game.state === 'game') router.replace(`/g/${props.code}/play`)
  if (store.game && store.game.state === 'finished') router.replace(`/g/${props.code}/results`)

  try {
    const qs = await playerApi.listQuestions(props.code)
    const mine = qs.find(q => q.userId === store.me?.id)
    if (mine) hydrateFromQuestion(mine)
  } catch {}

  await nextTick()
  initialReady.value = true
  clockTimer = setInterval(() => { nowMs.value = Date.now() }, 15_000)
})

onUnmounted(() => {
  if (clockTimer) clearInterval(clockTimer)
})

watch(() => store.game && store.game.state, (s) => {
  if (s === 'game') router.replace(`/g/${props.code}/play`)
  if (s === 'finished') router.replace(`/g/${props.code}/results`)
})

function hydrateFromQuestion(q: Question) {
  text.value = q.text
  photoId.value = q.photoImageId || ''
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
    const r = await playerApi.aiSuggest({
      hint: '',
      answerType: 'choice',
      photoImageId: photoId.value,
    })
    answerType.value = 'choice'
    text.value = r.text || ''
    if (Array.isArray(r.options) && r.options.length >= 2) {
      options.value = r.options.slice(0, 4)
      correctIdx.value = Number(r.correct) || 0
    }
    step.value = 'editor'
    armSaveHint()
  } catch (e) {
    err.value = 'AI: ' + errMsg(e, 'failed')
  } finally {
    aiBusy.value = false
  }
}

async function save() {
  dismissSaveHint()
  err.value = ''
  loading.value = true
  try {
    const body: {
      text: string
      photoImageId: string
      answerType: AnswerType
      options: string[]
      correct?: string | number
    } = {
      text: text.value.trim(),
      photoImageId: photoId.value,
      answerType: answerType.value,
      options: answerType.value === 'choice' ? options.value.map(o => o.trim()).filter(Boolean) : [],
      correct: undefined,
    }
    if (answerType.value === 'yesno') body.correct = correct.value
    else if (answerType.value === 'choice') body.correct = correctIdx.value
    else body.correct = Number(correctNumber.value)
    await playerApi.putQuestion(props.code, body)
    saved.value = true
    editing.value = false
  } catch (e) {
    err.value = errMsg(e, 'Could not save')
  } finally {
    loading.value = false
  }
}

async function saveLockedEmail() {
  emailErr.value = ''
  savingEmail.value = true
  try {
    const trimmed = lockedEmail.value.trim()
    await playerApi.updateMe({
      name: store.me?.name || '',
      photoImageId: store.me?.photoImageId || '',
      email: trimmed,
    })
    store.updateMe({ email: trimmed })
    emailSent.value = true
  } catch (e) {
    emailErr.value = errMsg(e, 'Could not save email')
  } finally {
    savingEmail.value = false
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
    const r = await playerApi.aiSuggest({
      hint: text.value || '',
      answerType: 'choice',
      photoImageId: photoId.value,
    })
    answerType.value = 'choice'
    text.value = r.text || text.value
    if (Array.isArray(r.options) && r.options.length >= 2) {
      options.value = r.options.slice(0, 4)
      correctIdx.value = Number(r.correct) || 0
    }
  } catch (e) {
    err.value = 'AI: ' + errMsg(e, 'failed')
  } finally {
    aiBusy.value = false
  }
}
</script>

<style scoped>
.wait-notice {
  margin-top: 12px;
  padding: 12px 14px;
  background: var(--yellow);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
  font-weight: 800;
  color: var(--ink);
  line-height: 1.35;
}

.email-offer {
  margin-top: 8px;
  padding: 14px;
  background: var(--paper, #fff);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
  text-align: left;
  width: 100%;
}
.email-offer__lead {
  margin: 0 0 8px;
  line-height: 1.35;
}
.email-offer__sent {
  font-weight: 800;
  color: var(--ink);
}
.save-hint {
  position: fixed;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 50;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 22px 28px;
  background: var(--yellow);
  color: var(--ink);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 6px 6px 0 var(--ink);
  font-weight: 800;
  font-size: 1.1rem;
  text-align: center;
  line-height: 1.3;
  cursor: pointer;
  max-width: calc(100vw - 32px);
  animation: save-hint-pop 220ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
.save-hint:hover:not(:disabled) {
  transform: translate(calc(-50% - 1px), calc(-50% - 1px));
  box-shadow: 7px 7px 0 var(--ink);
}
.save-hint:active:not(:disabled) {
  transform: translate(calc(-50% + 3px), calc(-50% + 3px));
  box-shadow: 3px 3px 0 var(--ink);
}
.save-hint__arrow {
  display: inline-block;
  font-size: 2rem;
  line-height: 1;
  animation: save-hint-bounce 1s infinite ease-in-out;
}
@keyframes save-hint-bounce {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(6px); }
}
@keyframes save-hint-pop {
  0% { opacity: 0; transform: translate(-50%, -50%) scale(0.7); }
  100% { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
@media (prefers-reduced-motion: reduce) {
  .save-hint__arrow,
  .save-hint { animation: none; }
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
