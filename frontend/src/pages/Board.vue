<template>
  <main class="board">
    <!-- Lobby: get 10 teams onto their phones -->
    <section v-if="state === 'setup'" class="board__lobby">
      <div class="board__lobby-left">
        <h1 class="board__title">{{ gameName || 'Trivia' }}</h1>
        <p class="board__kicker">Scan to join · one phone per team</p>
        <div class="board__join-url">{{ joinUrlShort }}</div>
        <div class="board__code">
          <span class="board__code-label">game code</span>
          <span class="board__code-value">{{ code }}</span>
        </div>
      </div>
      <div class="board__qr">
        <img v-if="qrDataUrl" :src="qrDataUrl" alt="QR code to join the game" />
        <div v-else class="board__qr-fallback">{{ joinUrl }}</div>
      </div>
      <div class="board__teams">
        <div class="board__teams-head">
          <h2>Teams in</h2>
          <span class="board__count">{{ users.length }}</span>
        </div>
        <ul class="board__team-list">
          <li v-for="u in users" :key="u.id">{{ u.name }}</li>
          <li v-if="!users.length" class="board__muted">Waiting for the first team…</li>
        </ul>
      </div>
    </section>

    <!-- Between questions -->
    <section v-else-if="state === 'game' && !question" class="board__center">
      <div class="board__spinner" aria-hidden="true"></div>
      <h1 class="board__title">Get ready…</h1>
      <p class="board__kicker">The host is lining up the next question.</p>
    </section>

    <!-- Live question -->
    <section v-else-if="state === 'game' && question" class="board__play">
      <header class="board__play-head">
        <span class="board__pill">
          Question {{ questionIndex }}<span v-if="totalQuestions"> / {{ totalQuestions }}</span>
        </span>
        <div v-if="questionState === 'active'" class="board__timer" :class="{ 'board__timer--low': remaining <= 10 }">
          {{ remaining }}s
        </div>
        <span v-else class="board__pill board__pill--reveal">What people said</span>
      </header>

      <h1 class="board__question">{{ question.text }}</h1>

      <!-- Answering: hide the options, show who has locked in -->
      <div v-if="questionState === 'active'" class="board__answering">
        <div class="board__lockins">
          <div
            v-for="u in users"
            :key="u.id"
            :class="['board__lockin', answeredIds.has(u.id) && 'board__lockin--in']"
          >
            <span class="board__lockin-tick" aria-hidden="true">{{ answeredIds.has(u.id) ? '✓' : '·' }}</span>
            {{ u.name }}
          </div>
        </div>
        <p class="board__kicker">
          {{ answeredCount }} of {{ users.length }} teams locked in
        </p>
      </div>

      <!-- Reveal: the board fills in from #5 up to #1 -->
      <ol v-else class="board__rows">
        <li
          v-for="(row, i) in revealRows"
          :key="row.index"
          class="board__row"
          :style="`--delay:${(revealRows.length - 1 - i) * 260}ms`"
        >
          <span class="board__row-rank">{{ row.rank }}</span>
          <span class="board__row-text">{{ row.text }}</span>
          <span class="board__row-pts">{{ row.points }}</span>
        </li>
      </ol>
    </section>

    <!-- Final -->
    <section v-else-if="state === 'finished'" class="board__center">
      <h1 class="board__title">🏆 {{ winnerName || 'That\'s a wrap' }}</h1>
      <p v-if="winnerName" class="board__kicker">wins IRL trivia with {{ leaderboard[0]?.points }} points</p>
    </section>

    <!-- Standings rail, always on once there is something to show -->
    <aside v-if="state !== 'setup' && leaderboard.length" class="board__standings">
      <h2>Standings</h2>
      <ol>
        <li v-for="(s, i) in leaderboard" :key="s.userId" :class="{ 'board__lead': i === 0 }">
          <span class="board__standings-rank">{{ i + 1 }}</span>
          <span class="board__standings-name">{{ s.userName }}</span>
          <span class="board__standings-pts">{{ s.points }}</span>
        </li>
      </ol>
    </aside>

    <footer v-if="!connected" class="board__offline">Reconnecting…</footer>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, toRef } from 'vue'
import QRCode from 'qrcode'
import { useGameStore } from '@/stores/game'
import { useQuestionCountdown } from '@/composables/useQuestionCountdown'
import type { PollOption } from '@/types'

const props = defineProps<{ code: string }>()
const store = useGameStore()

const qrDataUrl = ref('')

const { remaining } = useQuestionCountdown(
  toRef(store, 'game'),
  { serverClockOffsetMs: toRef(store, 'serverClockOffsetMs'), intervalMs: 250 },
)

const connected = computed(() => store.connected !== false)
const gameName = computed(() => store.game?.name || '')
const state = computed(() => store.game?.state || 'setup')
const questionState = computed(() => store.game?.questionState)
const question = computed(() => store.question)
const questionIndex = computed(() => store.game?.questionIndex || 0)
const totalQuestions = computed(() => store.game?.totalQuestions || 0)
const users = computed(() => store.users)
const leaderboard = computed(() => store.leaderboard)
const answeredIds = computed(() => new Set(store.answeredUserIds))
const answeredCount = computed(() => answeredIds.value.size)
const winnerName = computed(() => store.leaderboard[0]?.userName || '')

const joinUrl = computed(() => `${location.origin}/g/${props.code}/join`)
// Typed off a screen across a room, so drop the protocol — it is pure noise
// there and it is what makes the line wrap on a 16:9 board.
const joinUrlShort = computed(() => joinUrl.value.replace(/^https?:\/\//, ''))

// Ranked by survey count — that ranking is the payoff of the format, and the
// CSS staggers the rows in from the bottom so #1 lands last.
const revealRows = computed(() => {
  const q = store.question
  if (!q || q.answerType !== 'poll') return []
  return (q.options as PollOption[])
    .map((o, index) => ({ index, text: o.text, points: o.points ?? 0 }))
    .sort((a, b) => b.points - a.points)
    .map((row, i) => ({ ...row, rank: i + 1 }))
})

onMounted(async () => {
  store.ensureBoardWS(props.code)
  try {
    qrDataUrl.value = await QRCode.toDataURL(joinUrl.value, {
      width: 720,
      margin: 1,
      color: { dark: '#1a1a1a', light: '#ffffff' },
    })
  } catch {
    // Fall back to the printed URL — the board still works without the QR.
  }
})
</script>

<style scoped>
/* The board is the only view designed for a room rather than a hand, so it
   opts out of the app's mobile-first sizing entirely and scales off vw. */
.board {
  min-height: 100vh;
  padding: clamp(24px, 3.2vw, 72px);
  display: flex;
  flex-direction: column;
  gap: clamp(12px, 1.6vw, 28px);
  background: var(--paper, #fdfaf3);
  color: var(--ink, #1a1a1a);
  font-size: clamp(16px, 1.3vw, 26px);
}

.board__title {
  font-size: clamp(2.2rem, 5vw, 5rem);
  line-height: 1.02;
  margin: 0;
}
.board__kicker {
  font-size: clamp(1rem, 1.6vw, 1.8rem);
  opacity: .7;
  margin: 0;
}
.board__muted { opacity: .55; }

/* ---- lobby ----
   Two columns, not three: the join URL is the longest unbreakable string on the
   board and a third column squeezed it until it wrapped mid-word. The QR spans
   both rows so the teams list gets the full text column as it fills up. */
.board__lobby {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  grid-template-areas:
    "left  qr"
    "teams qr";
  gap: clamp(20px, 3vw, 56px);
  align-content: center;
}
.board__lobby-left {
  grid-area: left;
  display: flex;
  flex-direction: column;
  gap: clamp(10px, 1.4vw, 22px);
  min-width: 0;
}
.board__join-url {
  font-family: var(--font-ui, ui-monospace, monospace);
  font-weight: 900;
  font-size: clamp(1.1rem, 2.1vw, 2.4rem);
  /* Break only if it truly cannot fit, never mid-word by default. */
  overflow-wrap: anywhere;
}
.board__code {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.board__code-label {
  text-transform: uppercase;
  letter-spacing: .16em;
  font-size: clamp(.7rem, 1vw, 1.1rem);
  opacity: .6;
}
.board__code-value {
  font-weight: 900;
  font-size: clamp(2.4rem, 5.5vw, 6rem);
  letter-spacing: .06em;
  line-height: 1;
}
.board__qr {
  grid-area: qr;
  align-self: center;
  background: #fff;
  padding: clamp(10px, 1.2vw, 22px);
  border: 4px solid var(--ink, #1a1a1a);
  box-shadow: 8px 8px 0 var(--ink, #1a1a1a);
}
.board__qr img { display: block; width: clamp(220px, 30vw, 540px); height: auto; }
.board__qr-fallback { max-width: 26vw; word-break: break-all; font-weight: 700; }

.board__teams {
  grid-area: teams;
  display: flex;
  flex-direction: column;
  gap: clamp(8px, 1vw, 16px);
  min-width: 0;
}
.board__teams-head { display: flex; align-items: baseline; gap: 14px; }
.board__teams-head h2 { margin: 0; font-size: clamp(1.2rem, 2vw, 2.2rem); }
.board__count {
  font-weight: 900;
  font-size: clamp(1.4rem, 2.6vw, 3rem);
}
.board__team-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: clamp(.95rem, 1.4vw, 1.6rem);
}
.board__team-list li {
  padding: .3em .8em;
  border: 3px solid var(--ink, #1a1a1a);
  border-radius: 999px;
  font-weight: 700;
  animation: board-pop .3s ease-out;
}

/* ---- play ---- */
.board__center {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 18px;
  text-align: center;
}
.board__play { flex: 1; display: flex; flex-direction: column; gap: clamp(12px, 1.8vw, 30px); }
.board__play-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.board__pill {
  font-weight: 900;
  text-transform: uppercase;
  letter-spacing: .1em;
  font-size: clamp(.8rem, 1.2vw, 1.3rem);
  padding: .35em 1em;
  border: 3px solid var(--ink, #1a1a1a);
  border-radius: 999px;
}
.board__pill--reveal { background: var(--yellow, #ffe066); }
.board__timer {
  font-weight: 900;
  font-size: clamp(2rem, 4vw, 4rem);
  font-variant-numeric: tabular-nums;
}
.board__timer--low { color: var(--coral, #ff6b6b); animation: board-pulse .9s ease-in-out infinite; }
.board__question {
  font-size: clamp(1.8rem, 4.2vw, 4.4rem);
  line-height: 1.06;
  margin: 0;
}

.board__answering { flex: 1; display: flex; flex-direction: column; justify-content: center; gap: clamp(12px, 1.6vw, 28px); }
.board__lockins { display: flex; flex-wrap: wrap; gap: 12px; }
.board__lockin {
  padding: .35em 1em;
  border: 3px solid var(--ink, #1a1a1a);
  border-radius: 999px;
  font-weight: 700;
  font-size: clamp(1rem, 1.6vw, 1.8rem);
  opacity: .35;
  transition: opacity .2s ease, transform .2s ease, background .2s ease;
}
.board__lockin--in {
  opacity: 1;
  background: var(--mint, #b7f0c2);
  transform: translateY(-2px);
  box-shadow: 4px 4px 0 var(--ink, #1a1a1a);
}
.board__lockin-tick { margin-right: .5em; font-weight: 900; }

/* Fill the space between the question and the standings rail so a short answer
   list does not leave the bottom third of the screen empty. */
.board__rows {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: clamp(8px, 1vw, 16px);
}
.board__row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: clamp(12px, 1.6vw, 28px);
  padding: clamp(8px, 1.1vw, 20px) clamp(12px, 1.6vw, 28px);
  border: 4px solid var(--ink, #1a1a1a);
  background: var(--cream, #fff8e7);
  box-shadow: 6px 6px 0 var(--ink, #1a1a1a);
  font-size: clamp(1.2rem, 2.4vw, 2.8rem);
  font-weight: 800;
  animation: board-flip .38s ease-out both;
  animation-delay: var(--delay, 0ms);
}
.board__row:first-child { background: var(--yellow, #ffe066); }
.board__row-rank {
  font-weight: 900;
  width: 1.6em;
  text-align: center;
  opacity: .55;
}
.board__row-text { min-width: 0; }
.board__row-pts { font-variant-numeric: tabular-nums; font-weight: 900; }

/* ---- standings ---- */
.board__standings {
  border-top: 4px solid var(--ink, #1a1a1a);
  padding-top: clamp(8px, 1vw, 18px);
}
.board__standings h2 {
  margin: 0 0 8px;
  font-size: clamp(.9rem, 1.2vw, 1.3rem);
  text-transform: uppercase;
  letter-spacing: .14em;
  opacity: .6;
}
.board__standings ol {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: clamp(8px, 1.2vw, 20px);
}
.board__standings li {
  display: flex;
  align-items: baseline;
  gap: .5em;
  font-size: clamp(.95rem, 1.5vw, 1.7rem);
  font-weight: 700;
}
.board__standings .board__lead { color: var(--coral, #ff6b6b); font-weight: 900; }
.board__standings-rank { opacity: .5; font-variant-numeric: tabular-nums; }
.board__standings-pts { font-variant-numeric: tabular-nums; font-weight: 900; }

.board__offline {
  position: fixed;
  bottom: 12px;
  right: 12px;
  font-weight: 700;
  opacity: .6;
}

.board__spinner {
  width: 48px;
  height: 48px;
  border: 6px solid rgba(0, 0, 0, .12);
  border-top-color: var(--ink, #1a1a1a);
  border-radius: 50%;
  animation: board-spin .9s linear infinite;
}

@keyframes board-spin { to { transform: rotate(360deg); } }
@keyframes board-pop { from { transform: scale(.8); opacity: 0; } }
@keyframes board-pulse { 50% { opacity: .45; } }
@keyframes board-flip {
  from { transform: translateY(14px) rotateX(-70deg); opacity: 0; }
  to { transform: none; opacity: 1; }
}

@media (max-width: 900px) {
  .board__lobby {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas: "left" "qr" "teams";
    justify-items: start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .board__row, .board__team-list li { animation: none; }
  .board__timer--low { animation: none; }
}
</style>
