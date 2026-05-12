<template>
  <main class="stack">
    <div v-if="!q" class="card stack center">
      <div style="font-size: 3rem;">⏳</div>
      <h2>Waiting for host…</h2>
      <p class="muted">The host will activate the next question.</p>
    </div>

    <template v-else>
      <div class="card stack">
        <div class="row between">
          <span class="tag">{{ qState === 'revealed' ? 'Answer' : 'Question' }}</span>
          <span class="timer" v-if="qState === 'active'">{{ remaining }}s</span>
        </div>

        <div class="photo-frame">
          <img :src="q.photoB64" alt="question photo" />
        </div>

        <h2>{{ q.text }}</h2>

        <!-- Answering -->
        <template v-if="qState === 'active' && !ack">
          <div v-if="q.answerType === 'yesno'" class="grid-2">
            <button class="option-btn" @click="answer('yes')">Yes</button>
            <button class="option-btn" @click="answer('no')">No</button>
          </div>
          <div v-else-if="q.answerType === 'choice'" class="stack">
            <button
              v-for="(opt, i) in q.options"
              :key="i"
              class="option-btn"
              @click="answer(i)"
            >{{ opt }}</button>
          </div>
          <div v-else-if="q.answerType === 'number'" class="stack">
            <input v-model.number="numberGuess" type="number" step="any" placeholder="Your guess" />
            <button class="btn-primary" @click="answer(Number(numberGuess))" :disabled="numberGuess === ''">Submit</button>
          </div>
        </template>

        <div v-else-if="qState === 'active' && ack" class="card stack center" style="margin-top: 4px;">
          <strong>Answer locked in 🔒</strong>
          <span class="muted">You took {{ (ack.responseMs/1000).toFixed(1) }}s</span>
        </div>

        <!-- Reveal -->
        <template v-if="qState === 'revealed'">
          <div v-if="q.answerType === 'yesno'" class="grid-2">
            <button :class="['option-btn', correctYes ? 'correct' : '']">Yes</button>
            <button :class="['option-btn', !correctYes ? 'correct' : '']">No</button>
          </div>
          <div v-else-if="q.answerType === 'choice'" class="stack">
            <button
              v-for="(opt, i) in q.options"
              :key="i"
              :class="['option-btn', i === correctIndex ? 'correct' : '']"
            >{{ opt }}</button>
          </div>
          <div v-else-if="q.answerType === 'number'" class="card stack center">
            <span class="muted">Correct answer</span>
            <h2 style="font-size: 2rem;">{{ correctNumber }}</h2>
          </div>
        </template>
      </div>

      <div v-if="qState === 'revealed'" class="card">
        <h2>Round results</h2>
        <ul class="ladder">
          <li v-for="a in answersWithUsers" :key="a.id" :class="{ me: a.userId === myId }">
            <img class="avatar" :src="a.photo" :alt="a.name" />
            <span>{{ a.name }}</span>
            <span class="muted" style="margin-left: 6px;">{{ (a.responseMs/1000).toFixed(1) }}s</span>
            <span class="pts">{{ a.isCorrect ? '✓' : '✗' }} {{ a.points }}</span>
          </li>
          <li v-if="answersWithUsers.length === 0" class="muted">No answers submitted.</li>
        </ul>
      </div>

      <div v-if="qState === 'revealed'" class="card">
        <h2>Leaderboard</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId" :class="{ me: s.userId === myId }">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span>{{ s.userName }}</span>
            <span class="pts">{{ s.points }} pts</span>
          </li>
        </ol>
      </div>
    </template>
  </main>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useGameStore } from '../stores/game.js'
import { wsSend } from '../services/ws.js'
import { api } from '../services/api.js'

const props = defineProps({ code: String })
const router = useRouter()
const store = useGameStore()

const numberGuess = ref('')
const remaining = ref(30)
let tickHandle = null

const q = computed(() => store.question)
const qState = computed(() => store.game && store.game.questionState)
const ack = computed(() => store.lastAnswerAck && q.value && store.lastAnswerAck.questionId === q.value.id ? store.lastAnswerAck : null)
const leaderboard = computed(() => store.leaderboard)
const myId = computed(() => store.me && store.me.id)

const correctYes = computed(() => q.value && q.value.correct === 'yes')
const correctIndex = computed(() => q.value ? Number(q.value.correct) : -1)
const correctNumber = computed(() => q.value ? Number(q.value.correct) : 0)

const answersWithUsers = computed(() => {
  const usersByID = Object.fromEntries((store.users || []).map(u => [u.id, u]))
  return (store.answers || []).map(a => ({
    ...a,
    name: usersByID[a.userId]?.name || '...',
    photo: usersByID[a.userId]?.photoB64 || '',
  }))
})

onMounted(async () => {
  await store.loadMe()
  if (!store.me) { router.replace('/'); return }
  store.ensureWS()
  // Pull initial users for name lookup
  try {
    store.users = await api.listUsers(props.code)
  } catch {}
})

watch(() => store.game && store.game.state, (s) => {
  if (s === 'setup') router.replace(`/g/${props.code}/setup`)
  if (s === 'finished') router.replace(`/g/${props.code}/results`)
})

watch(() => [qState.value, store.game && store.game.questionStartedAt], () => {
  resetTick()
})

function resetTick() {
  if (tickHandle) { clearInterval(tickHandle); tickHandle = null }
  numberGuess.value = ''
  if (qState.value !== 'active') return
  const startedAt = store.game && store.game.questionStartedAt ? new Date(store.game.questionStartedAt).getTime() : Date.now()
  const tick = () => {
    const elapsed = (Date.now() - startedAt) / 1000
    remaining.value = Math.max(0, 30 - Math.floor(elapsed))
  }
  tick()
  tickHandle = setInterval(tick, 250)
}

function answer(v) {
  if (!q.value) return
  wsSend('answer', { questionId: q.value.id, value: v })
}
</script>
