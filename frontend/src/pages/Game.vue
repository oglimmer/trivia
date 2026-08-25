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
        <button type="button" class="btn-ghost" @click="scoringInfoOpen = true">
          🎯 How scoring works
        </button>
      </div>
    </template>

    <template v-else>
      <article class="q-card stack">
        <header class="row between">
          <span :class="['tag', qState === 'revealed' ? 'tag--mint' : 'tag--pink']">
            {{ qState === 'revealed' ? '✓ Answer' : 'Question' }}
            <template v-if="questionIndex && totalQuestions"> {{ questionIndex }} / {{ totalQuestions }}</template>
          </span>
          <div v-if="qState === 'active'" class="timer-ring" :style="`--pct:${ringPct}`" aria-label="time remaining">
            <span>{{ remaining }}</span>
          </div>
        </header>

        <div v-if="q.photoImageId" class="photo-frame">
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
          <div v-else-if="q.answerType === 'poll'" class="stack">
            <div class="muted center bold" style="font-size: .85rem; letter-spacing: .04em;">
              👥 What did most people say?
            </div>
            <button
              v-for="(opt, i) in pollOptions"
              :key="i"
              class="option-btn"
              @click="answer(i)"
            >
              <span class="option-btn__bullet">{{ letters[i] }}</span>{{ opt.text }}
            </button>
          </div>
          <div v-else-if="q.answerType === 'number'" class="stack">
            <div class="muted center bold" style="font-size: .85rem; letter-spacing: .04em;">🔢 Type in a number</div>
            <input
              v-model.number="numberGuess"
              type="number"
              inputmode="decimal"
              step="any"
              placeholder="e.g. 42"
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
          <div v-else-if="q.answerType === 'poll'" class="stack">
            <div class="muted center bold" style="font-size: .85rem; letter-spacing: .04em;">
              WHAT PEOPLE SAID
            </div>
            <div
              v-for="row in pollBoard"
              :key="row.index"
              :class="['option-btn', 'poll-row', row.index === myPollPick && 'correct']"
            >
              <span class="option-btn__bullet">{{ row.rank }}</span>
              <span class="poll-row__text">{{ row.text }}</span>
              <span class="poll-row__pts">{{ row.points }}</span>
            </div>
          </div>
          <div v-else-if="q.answerType === 'number'" class="card card--yellow center">
            <span class="muted" style="font-weight: 700; letter-spacing: .12em; text-transform: uppercase; font-size: .8rem;">Correct answer</span>
            <div style="font-family: var(--font-ui); font-weight: 900; font-size: 3rem; line-height: 1; margin-top: 6px;">{{ correctNumber }}</div>
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

      <div v-if="qState === 'revealed' && leaderboardHidden" class="card card--yellow center stack">
        <div style="font-size: 2rem;">🤐</div>
        <div class="bold" style="font-size: 1.1rem;">Scores hidden — final stretch!</div>
        <div class="muted">Standings are revealed when the game ends.</div>
      </div>
      <div v-else-if="qState === 'revealed'" class="card">
        <h2>Leaderboard</h2>
        <Leaderboard :entries="leaderboard" :my-id="myId || undefined" />
      </div>
    </template>

    <ScoringInfoDialog :open="scoringInfoOpen" @close="scoringInfoOpen = false" />
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
import { useWrongSoundEffect } from '@/composables/useWrongSoundEffect'
import { pickVerdictLine } from '@/utils/verdict'
import type { PollOption } from '@/types'
import Leaderboard from '@/components/Leaderboard.vue'
import ScoringInfoDialog from '@/components/ScoringInfoDialog.vue'

const wrongSound = useWrongSoundEffect()

const props = defineProps<{ code: string }>()
const router = useRouter()
const store = useGameStore()

const numberGuess = ref<number | ''>('')
const letters = ['A', 'B', 'C', 'D', 'E']
const initialReady = ref(false)
const animateVerdict = ref(false)
const scoringInfoOpen = ref(false)
let pageLoaded = false
let stopListen: (() => void) | null = null

const { remaining, ringPct } = useQuestionCountdown(
  toRef(store, 'game'),
  { serverClockOffsetMs: toRef(store, 'serverClockOffsetMs') },
)

const q = computed(() => store.question)
const qState = computed(() => store.game && store.game.questionState)
const questionIndex = computed(() => store.game?.questionIndex || 0)
const totalQuestions = computed(() => store.game?.totalQuestions || 0)
const leaderboardHidden = computed(() => !!store.game?.leaderboardHidden)
const ack = computed(() => store.lastAnswerAck && q.value && store.lastAnswerAck.questionId === q.value.id ? store.lastAnswerAck : null)
const leaderboard = computed(() => store.leaderboard)
const myId = computed(() => store.me && store.me.id)

// Poll options arrive as {text, points}; points are withheld until the reveal.
const pollOptions = computed<PollOption[]>(() => {
  if (!q.value || q.value.answerType !== 'poll') return []
  return (q.value.options || []) as PollOption[]
})

const myPollPick = computed(() => {
  const raw = myAnswer.value?.answer
  return typeof raw === 'number' ? raw : -1
})

// The reveal board is ranked by survey count, not by the order the options were
// shown in — that ranking is the payoff of the format.
const pollBoard = computed(() => {
  return pollOptions.value
    .map((o, index) => ({ index, text: o.text, points: o.points ?? 0 }))
    .sort((a, b) => b.points - a.points)
    .map((row, i) => ({ ...row, rank: i + 1 }))
})

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

const verdict = computed(() => {
  const seed = (q.value && q.value.id) || ''
  if (q.value?.answerType === 'poll') return pollVerdict.value
  if (!myAnswer.value) {
    return { kind: 'none' as const, emoji: '👻', ...pickVerdictLine('none', seed) }
  }
  if (myAnswer.value.isCorrect) {
    return { kind: 'correct' as const, emoji: '🎉', ...pickVerdictLine('correct', seed) }
  }
  return { kind: 'wrong' as const, emoji: '💥', ...pickVerdictLine('wrong', seed) }
})

// Nothing is "wrong" in a poll — every listed answer scores. The verdict says
// where the team landed against the crowd instead of passing judgement.
const pollVerdict = computed(() => {
  const seed = (q.value && q.value.id) || ''
  if (!myAnswer.value) {
    return { kind: 'none' as const, emoji: '👻', ...pickVerdictLine('none', seed) }
  }
  const top = pollBoard.value[0]
  const mine = pollBoard.value.find(r => r.index === myPollPick.value)
  if (!mine) {
    return { kind: 'none' as const, emoji: '👻', ...pickVerdictLine('none', seed) }
  }
  if (mine.rank === 1) {
    return {
      kind: 'correct' as const,
      emoji: '🥇',
      headline: 'TOP ANSWER!',
      sub: `${mine.points} people said "${mine.text}". So did you.`,
    }
  }
  return {
    kind: 'correct' as const,
    emoji: '📊',
    headline: `#${mine.rank} ON THE BOARD`,
    sub: `"${mine.text}" scored ${mine.points}. Top was "${top?.text}" with ${top?.points}.`,
  }
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

watch(qState, (newVal) => {
  if (!pageLoaded) return
  // No losing sound in poll mode: every option on the board scores, so the
  // sting would fire on a perfectly good answer.
  if (newVal === 'revealed' && verdict.value.kind === 'wrong' && q.value?.answerType !== 'poll') {
    wrongSound.play()
  }
}, { flush: 'post' })

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
