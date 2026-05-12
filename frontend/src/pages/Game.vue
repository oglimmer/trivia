<template>
  <main class="stack-lg">
    <div v-if="!q" class="card card--cream stack center card-stickered">
      <div class="spinner" aria-hidden="true"></div>
      <h2>Waiting for the host…</h2>
      <p class="muted">The next question is about to drop.</p>
    </div>

    <template v-else>
      <article class="q-card stack">
        <header class="row between">
          <span :class="['tag', qState === 'revealed' ? 'tag--mint' : 'tag--pink']">
            {{ qState === 'revealed' ? '✓ Answer' : 'Question' }}
          </span>
          <div v-if="qState === 'active'" class="timer-ring" :style="`--pct:${ringPct}`" aria-label="time remaining">
            <span>{{ remaining }}</span>
          </div>
        </header>

        <div class="photo-frame">
          <img :src="q.photoB64" alt="question photo" />
          <img
            v-if="authorPhoto"
            class="q-author-avatar"
            :src="authorPhoto"
            :alt="authorName ? `by ${authorName}` : ''"
            :title="authorName ? `by ${authorName}` : ''"
          />
        </div>

        <h2 class="q-card__text">{{ q.text }}</h2>

        <!-- Answering -->
        <template v-if="qState === 'active' && !ack">
          <div v-if="q.answerType === 'yesno'" class="grid-2">
            <button class="option-btn" @click="answer('yes')">
              <span class="option-btn__bullet">Y</span>Yes
            </button>
            <button class="option-btn" @click="answer('no')">
              <span class="option-btn__bullet">N</span>No
            </button>
          </div>
          <div v-else-if="q.answerType === 'choice'" class="stack">
            <button
              v-for="(opt, i) in q.options"
              :key="i"
              class="option-btn"
              @click="answer(i)"
            >
              <span class="option-btn__bullet">{{ letters[i] }}</span>{{ opt }}
            </button>
          </div>
          <div v-else-if="q.answerType === 'number'" class="stack">
            <input
              v-model.number="numberGuess"
              type="number"
              step="any"
              placeholder="Your best guess"
              style="font-size: 1.4rem; text-align: center;"
            />
            <button class="btn-primary btn-lg btn-block" @click="answer(Number(numberGuess))" :disabled="numberGuess === ''">
              Lock it in
            </button>
          </div>
        </template>

        <div v-else-if="qState === 'active' && ack" class="card card--mint center stack" style="margin-top: 4px;">
          <div style="font-size: 2rem;">🔒</div>
          <div class="bold" style="font-size: 1.2rem;">Locked in!</div>
          <div class="muted">Took {{ (ack.responseMs / 1000).toFixed(1) }}s — waiting for the others…</div>
        </div>

        <!-- Reveal -->
        <template v-if="qState === 'revealed'">
          <div v-if="q.answerType === 'yesno'" class="grid-2">
            <div :class="['option-btn', correctYes && 'correct', !correctYes && 'wrong']">
              <span class="option-btn__bullet">Y</span>Yes
            </div>
            <div :class="['option-btn', !correctYes && 'correct', correctYes && 'wrong']">
              <span class="option-btn__bullet">N</span>No
            </div>
          </div>
          <div v-else-if="q.answerType === 'choice'" class="stack">
            <div
              v-for="(opt, i) in q.options"
              :key="i"
              :class="['option-btn', i === correctIndex ? 'correct' : 'wrong']"
            >
              <span class="option-btn__bullet">{{ letters[i] }}</span>{{ opt }}
            </div>
          </div>
          <div v-else-if="q.answerType === 'number'" class="card card--yellow center">
            <span class="muted" style="font-weight: 700; letter-spacing: .12em; text-transform: uppercase; font-size: .8rem;">Correct answer</span>
            <div style="font-family: var(--font-display); font-style: italic; font-weight: 900; font-size: 3rem; line-height: 1; margin-top: 6px;">{{ correctNumber }}</div>
          </div>
        </template>
      </article>

      <div v-if="qState === 'revealed'" class="card card--cream">
        <div class="row between" style="margin-bottom: 12px;">
          <h2 style="margin: 0;">Round</h2>
          <span class="tag tag--blue">{{ answersWithUsers.length }} answers</span>
        </div>
        <ul class="ladder">
          <li v-for="a in answersWithUsers" :key="a.id" :class="{ me: a.userId === myId }">
            <img class="avatar" :src="a.photo" :alt="a.name" />
            <span class="bold">{{ a.name }}</span>
            <span class="muted timer">{{ (a.responseMs / 1000).toFixed(1) }}s</span>
            <span class="pts">
              <span :style="`color: ${a.isCorrect ? 'var(--mint)' : 'var(--coral)'};`">{{ a.isCorrect ? '✓' : '✗' }}</span>
              {{ a.points }}
            </span>
          </li>
          <li v-if="answersWithUsers.length === 0" class="muted center" style="justify-content: center;">No answers submitted.</li>
        </ul>
      </div>

      <div v-if="qState === 'revealed'" class="card">
        <h2>Leaderboard</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId" :class="{ me: s.userId === myId }">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span class="bold">{{ s.userName }}</span>
            <span class="pts">{{ s.points }}</span>
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
const ringPct = ref(100)
let tickHandle = null
const letters = ['A', 'B', 'C', 'D']

const q = computed(() => store.question)
const qState = computed(() => store.game && store.game.questionState)
const ack = computed(() => store.lastAnswerAck && q.value && store.lastAnswerAck.questionId === q.value.id ? store.lastAnswerAck : null)
const leaderboard = computed(() => store.leaderboard)
const myId = computed(() => store.me && store.me.id)

const correctYes = computed(() => q.value && q.value.correct === 'yes')
const correctIndex = computed(() => q.value ? Number(q.value.correct) : -1)
const correctNumber = computed(() => q.value ? Number(q.value.correct) : 0)

const usersByID = computed(() => Object.fromEntries((store.users || []).map(u => [u.id, u])))
const authorName = computed(() => q.value ? (usersByID.value[q.value.userId]?.name || '') : '')
const authorPhoto = computed(() => q.value ? (usersByID.value[q.value.userId]?.photoB64 || '') : '')

const answersWithUsers = computed(() => {
  return (store.answers || []).map(a => ({
    ...a,
    name: usersByID.value[a.userId]?.name || '...',
    photo: usersByID.value[a.userId]?.photoB64 || '',
  }))
})

onMounted(async () => {
  await store.loadMe()
  if (!store.me) { router.replace('/'); return }
  store.ensureWS()
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
    const left = Math.max(0, 30 - elapsed)
    remaining.value = Math.max(0, Math.ceil(left))
    ringPct.value = Math.round((left / 30) * 100)
  }
  tick()
  tickHandle = setInterval(tick, 200)
}

function answer(v) {
  if (!q.value) return
  wsSend('answer', { questionId: q.value.id, value: v })
}
</script>
