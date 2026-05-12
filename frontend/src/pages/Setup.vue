<template>
  <main class="stack-lg">
    <transition name="fade" mode="out-in">
      <div v-if="saved && !editing" key="waiting" class="card card--mint stack center card-stickered">
        <div style="font-size: 3rem; line-height: 1;">🎉</div>
        <h1>Locked in!</h1>
        <p>Your question is ready. Sit tight — the host kicks things off in a moment.</p>
        <button class="btn-ghost" @click="editing = true">← Edit my question</button>
      </div>

      <div v-else key="editing" class="card stack">
        <div class="row between">
          <h1 style="margin: 0;">Your question</h1>
          <span class="tag tag--yellow">1 each</span>
        </div>
        <p class="muted" style="margin-top: -4px;">Snap a photo, write a question, set the right answer.</p>

        <label>Photo</label>
        <PhotoPicker v-model="photo" />

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
          <button class="btn-blue btn-block" @click="requestAI" :disabled="!photo || aiBusy">
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
          <button v-if="saved" class="btn-ghost" @click="editing = false">Cancel</button>
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

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import PhotoPicker from '../components/PhotoPicker.vue'
import { api } from '../services/api.js'
import { useGameStore } from '../stores/game.js'

const props = defineProps({ code: String })
const router = useRouter()
const store = useGameStore()

const photo = ref('')
const text = ref('')
const answerType = ref('yesno')
const correct = ref('yes')           // yesno
const options = ref(['', ''])        // choice
const correctIdx = ref(0)
const correctNumber = ref(0)
const loading = ref(false)
const saved = ref(false)
const editing = ref(false)
const err = ref('')
const aiBusy = ref(false)
const aiConfirm = ref(false)

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
    const mine = qs.find(q => q.userId === store.me.id)
    if (mine) hydrateFromQuestion(mine)
  } catch {}
})

watch(() => store.game && store.game.state, (s) => {
  if (s === 'game') router.replace(`/g/${props.code}/play`)
  if (s === 'finished') router.replace(`/g/${props.code}/results`)
})

function hydrateFromQuestion(q) {
  text.value = q.text
  photo.value = q.photoB64
  answerType.value = q.answerType
  if (q.answerType === 'choice') {
    options.value = Array.isArray(q.options) && q.options.length ? q.options : ['', '']
  }
  saved.value = true
}

function removeOption(i) {
  options.value.splice(i, 1)
  if (correctIdx.value >= options.value.length) correctIdx.value = 0
}

async function save() {
  err.value = ''
  loading.value = true
  try {
    const body = {
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
    err.value = e.message || 'Could not save'
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
    err.value = 'AI: ' + (e.message || 'failed')
  } finally {
    aiBusy.value = false
  }
}
</script>
