<template>
  <main class="stack-lg">
    <div v-if="!initialReady" class="card card--cream stack center" aria-busy="true">
      <div class="spinner" aria-hidden="true"></div>
    </div>

    <template v-else-if="!q">
      <div class="card card--cream stack center card-stickered">
        <div class="spinner" aria-hidden="true"></div>
        <h2>Waiting for the host…</h2>
        <p class="muted">The next question is about to drop.</p>
      </div>
    </template>

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
          <img
            :src="imageUrl(q.photoImageId, 'medium')"
            alt="question photo"
            loading="lazy"
            decoding="async"
          />
          <img
            v-if="authorPhoto"
            class="q-author-avatar"
            :src="authorPhoto"
            :alt="authorName ? `by ${authorName}` : ''"
            :title="authorName ? `by ${authorName}` : ''"
            loading="lazy"
            decoding="async"
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
          <div :class="['verdict', `verdict--${verdict.kind}`, animateVerdict && `verdict-anim--${verdict.kind}`]" role="status" aria-live="polite">
            <div class="verdict__stamp" aria-hidden="true">{{ verdict.emoji }}</div>
            <div class="verdict__text">
              <div class="verdict__headline">{{ verdict.headline }}</div>
              <div class="verdict__sub">{{ verdict.sub }}</div>
            </div>
          </div>
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
            <img class="avatar" :src="a.photo" :alt="a.name" loading="lazy" decoding="async" />
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
            <img class="avatar" :src="imageUrl(s.photoImageId, 'thumb')" :alt="s.userName" loading="lazy" decoding="async" />
            <span class="bold">{{ s.userName }}</span>
            <span class="pts">{{ s.points }}</span>
          </li>
        </ol>
      </div>
    </template>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch, toRef } from 'vue'
import { useRouter } from 'vue-router'
import { useGameStore } from '@/stores/game'
import { onMessage, wsSend } from '@/services/ws'
import { playerApi } from '@/services/api'
import { imageUrl } from '@/services/images'
import { useQuestionCountdown } from '@/composables/useQuestionCountdown'

type VerdictKind = 'correct' | 'wrong' | 'none'

const props = defineProps<{ code: string }>()
const router = useRouter()
const store = useGameStore()

const numberGuess = ref<number | ''>('')
const letters = ['A', 'B', 'C', 'D']
const initialReady = ref(false)
const animateVerdict = ref(false)
let pageLoaded = false
let stopListen: (() => void) | null = null

const { remaining, ringPct } = useQuestionCountdown(
  toRef(store, 'game'),
  { serverClockOffsetMs: toRef(store, 'serverClockOffsetMs') },
)

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
const authorPhoto = computed(() => {
  if (!q.value) return ''
  const u = usersByID.value[q.value.userId]
  if (!u) return ''
  return imageUrl(u.photoImageId, 'thumb')
})

const answersWithUsers = computed(() => {
  return (store.answers || []).map(a => {
    const u = usersByID.value[a.userId]
    return {
      ...a,
      name: u?.name || '...',
      photo: imageUrl(u?.photoImageId, 'thumb'),
    }
  }).sort((a, b) => b.points - a.points || a.responseMs - b.responseMs)
})

const myAnswer = computed(() => (store.answers || []).find(a => a.userId === myId.value))

const verdictLines: Record<VerdictKind, Array<{ headline: string; sub: string }>> = {
  correct: [
    { headline: 'NAILED IT!', sub: 'Big brain energy detected.' },
    { headline: 'CORRECT!', sub: 'Frame this moment. Tell your mum.' },
    { headline: 'BOOM!', sub: 'You absolute trivia gremlin.' },
    { headline: 'CHEF’S KISS', sub: 'Smooth. Effortless. Annoying.' },
    { headline: 'TOO EASY', sub: 'Were you peeking? You were peeking.' },
  ],
  wrong: [
    { headline: 'NOPE.', sub: 'Confidently incorrect. Respect.' },
    { headline: 'OOF.', sub: 'That answer ate gravel.' },
    { headline: 'SWING AND A MISS', sub: 'Points for vibes only.' },
    { headline: 'NOT QUITE.', sub: 'Geographically near. Factually no.' },
    { headline: 'YIKES!', sub: 'Even the dog would’ve guessed better.' },
  ],
  none: [
    { headline: 'GHOSTED.', sub: 'You said nothing. Loudly.' },
    { headline: 'NO ANSWER?', sub: 'Bold strategy. Zero points.' },
    { headline: 'AWOL', sub: 'We waited. You vibed elsewhere.' },
  ],
}

function pickLine(kind: VerdictKind, seed: string) {
  const lines = verdictLines[kind]
  let h = 0
  const s = String(seed || '')
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return lines[h % lines.length]
}

const verdict = computed(() => {
  const seed = (q.value && q.value.id) || ''
  if (!myAnswer.value) {
    return { kind: 'none' as const, emoji: '👻', ...pickLine('none', seed) }
  }
  if (myAnswer.value.isCorrect) {
    return { kind: 'correct' as const, emoji: '🎉', ...pickLine('correct', seed) }
  }
  return { kind: 'wrong' as const, emoji: '💥', ...pickLine('wrong', seed) }
})

async function markReady() {
  await nextTick()
  initialReady.value = true
  pageLoaded = true
}

onMounted(async () => {
  await store.loadMe()
  if (!store.me) { router.replace('/'); return }
  store.ensureWS()
  try {
    store.setUsers(await playerApi.listUsers(props.code))
  } catch {}

  // If we already have authoritative state (navigated from setup, or the state
  // is 'idle' which doesn't need a question payload), unlock immediately.
  const needsQuestion = qState.value === 'active' || qState.value === 'revealed'
  if (!needsQuestion || q.value) {
    await markReady()
    return
  }

  // Otherwise, wait for the first WS gameState message that brings the question.
  stopListen = onMessage((m) => {
    if (m.type === 'gameState') {
      if (stopListen) { stopListen(); stopListen = null }
      void markReady()
    }
  })
})

onUnmounted(() => {
  if (stopListen) stopListen()
})

// Only animate the verdict pop on real state transitions during play, not on
// the initial mount when reloading mid-reveal.
watch(qState, (newVal) => {
  if (!pageLoaded) return
  animateVerdict.value = newVal === 'revealed'
}, { flush: 'pre' })

watch(() => store.game && store.game.state, (s) => {
  if (s === 'setup') router.replace(`/g/${props.code}/setup`)
  if (s === 'finished') router.replace(`/g/${props.code}/results`)
})

// Clear the user's number input each time a new question starts.
watch(() => [qState.value, store.game?.questionStartedAt], () => {
  numberGuess.value = ''
})

function answer(v: unknown) {
  if (!q.value) return
  wsSend('answer', { questionId: q.value.id, value: v })
}
</script>
