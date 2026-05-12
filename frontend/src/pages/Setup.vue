<template>
  <main class="stack">
    <transition name="fade" mode="out-in">
      <div v-if="saved && !editing" key="waiting" class="card stack center">
        <h1>🎉 You're in!</h1>
        <p>Your question is locked in. Sit tight — the host will start the game soon.</p>
        <button class="btn-ghost" @click="editing = true">← Edit my question</button>
      </div>

      <div v-else key="editing" class="card stack">
        <h1>Add your question</h1>
        <p class="muted">Upload a photo, write a question, and define the answers. Everyone gets one.</p>

        <label>Photo</label>
        <PhotoPicker v-model="photo" />

        <label>Question</label>
        <textarea v-model="text" placeholder="What is this?" maxlength="160" rows="3"></textarea>

        <label>Answer type</label>
        <select v-model="answerType">
          <option value="yesno">Yes / No</option>
          <option value="choice">Multiple choice (2-4)</option>
          <option value="number">Guess a number</option>
        </select>

        <template v-if="answerType === 'yesno'">
          <label>Correct answer</label>
          <div class="grid-2">
            <button :class="['option-btn', correct === 'yes' && 'chosen']" @click="correct = 'yes'">Yes</button>
            <button :class="['option-btn', correct === 'no'  && 'chosen']" @click="correct = 'no'">No</button>
          </div>
        </template>

        <template v-if="answerType === 'choice'">
          <label>Options (2-4) — tap the one that's correct</label>
          <div class="stack">
            <div v-for="(_, i) in options" :key="i" class="row">
              <input
                v-model="options[i]"
                :placeholder="`Option ${i+1}`"
                maxlength="60"
                :class="{ 'chosen': correctIdx === i }"
              />
              <button
                :class="['option-btn', correctIdx === i && 'correct']"
                style="width: auto; padding: 8px 12px;"
                @click="correctIdx = i"
                :aria-label="`Mark option ${i+1} as correct`"
              >✓</button>
              <button v-if="options.length > 2" class="btn-danger" style="padding: 8px 12px;" @click="removeOption(i)">✕</button>
            </div>
            <button v-if="options.length < 4" @click="options.push('')">+ Add option</button>
          </div>
        </template>

        <template v-if="answerType === 'number'">
          <label>Correct number</label>
          <input v-model.number="correctNumber" type="number" step="any" placeholder="42" />
        </template>

        <div v-if="!aiConfirm" class="row">
          <button @click="requestAI" :disabled="!photo || aiBusy">
            {{ aiBusy ? '✨ Thinking…' : '✨ Help me with AI' }}
          </button>
        </div>
        <div v-else class="card stack" style="background: var(--surface-2); padding: 12px;">
          <p style="margin: 0;">AI will replace your current question{{ extraFieldsLabel }}. Continue?</p>
          <div class="row">
            <button class="btn-primary" style="flex: 1;" @click="confirmAI">Replace</button>
            <button class="btn-ghost" @click="aiConfirm = false">Keep mine</button>
          </div>
        </div>

        <div class="row">
          <button class="btn-primary" style="flex: 1;" :disabled="!canSubmit || loading" @click="save">
            {{ loading ? 'Saving…' : (saved ? 'Update question' : 'Save question') }}
          </button>
          <button v-if="saved" class="btn-ghost" @click="editing = false">Cancel</button>
        </div>
        <div v-if="err" class="error">{{ err }}</div>
      </div>
    </transition>

    <transition name="fade">
      <div v-if="aiBusy" class="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="ai-busy-title">
        <div class="modal card stack center">
          <div class="spinner" aria-hidden="true"></div>
          <h2 id="ai-busy-title" style="margin: 0;">✨ Working on it…</h2>
          <p class="muted" style="margin: 0;">AI is crafting your question. This usually takes a few seconds.</p>
        </div>
      </div>
    </transition>

    <div class="card">
      <div class="row between" style="margin-bottom: 8px;">
        <h2>Players</h2>
        <span class="tag">{{ users.length }}</span>
      </div>
      <div class="row" style="flex-wrap: wrap; gap: 10px;">
        <div v-for="u in users" :key="u.id" class="row" style="gap: 6px;">
          <img class="avatar" :src="u.photoB64 || ''" :alt="u.name" />
          <span>{{ u.name }}</span>
        </div>
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
const options = ref(['', ''])         // choice
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
  // If state changed (admin started the game) before we mount, jump:
  if (store.game && store.game.state === 'game') router.replace(`/g/${props.code}/play`)
  if (store.game && store.game.state === 'finished') router.replace(`/g/${props.code}/results`)

  // Preload my existing question if any:
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
      answerType: answerType.value,
      photoB64: photo.value,
    })
    text.value = r.text || text.value
    if (answerType.value === 'choice' && Array.isArray(r.options) && r.options.length >= 2) {
      options.value = r.options.slice(0, 4)
      correctIdx.value = Number(r.correct) || 0
    } else if (answerType.value === 'yesno') {
      correct.value = (r.correct === 'no' ? 'no' : 'yes')
    } else if (answerType.value === 'number') {
      correctNumber.value = Number(r.correct) || 0
    }
  } catch (e) {
    err.value = 'AI: ' + (e.message || 'failed')
  } finally {
    aiBusy.value = false
  }
}
</script>
