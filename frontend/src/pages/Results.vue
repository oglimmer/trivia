<template>
  <main class="stack-lg">
    <transition :name="initialReady ? 'fade' : ''" mode="out-in">
      <div :key="phase" v-if="leaderboard.length">
        <template v-if="phase === 'three'">
          <div class="card stack">
            <h2 class="center" style="margin: 0;">In third place…</h2>
            <Spotlight :score="byRank[2]" rank="3" color="bronze" />
            <button class="btn-primary btn-lg btn-block" @click="phase = 'two'">Next →</button>
          </div>
        </template>

        <template v-else-if="phase === 'two'">
          <div class="card stack">
            <h2 class="center" style="margin: 0;">In second place…</h2>
            <Spotlight :score="byRank[1]" rank="2" color="silver" />
            <button class="btn-primary btn-lg btn-block" @click="phase = 'one'">Next →</button>
          </div>
        </template>

        <template v-else-if="phase === 'one'">
          <div class="card stack">
            <h2 class="center" style="margin: 0;">The winner is…</h2>
            <Spotlight :score="byRank[0]" rank="1" color="gold" big />
            <button class="btn-warn btn-lg btn-block" @click="phase = 'ladder'">Show full ladder →</button>
          </div>
        </template>

        <template v-else>
          <div class="card stack">
            <h1 style="margin: 0;">Final standings</h1>
            <div class="toggles results-tabs">
              <button
                type="button"
                :class="{ active: view === 'standings' }"
                @click="view = 'standings'"
              >Standings</button>
              <button
                type="button"
                class="breakdown-tab"
                :class="{ active: view === 'breakdown' }"
                @click="setView('breakdown')"
              >
                Question breakdown
                <span v-if="!myVote" class="vote-badge">★ Vote</span>
              </button>
            </div>
            <button
              v-if="view === 'standings' && !myVote"
              type="button"
              class="vote-callout"
              @click="setView('breakdown')"
            >
              <span class="vote-callout__star" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2l2.9 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l7.1-1.01z" />
                </svg>
              </span>
              <span class="vote-callout__body">
                <strong class="vote-callout__title">Vote for the best question!</strong>
                <span class="vote-callout__sub">Open the question breakdown to crown your favourite — you get one vote.</span>
              </span>
              <span class="vote-callout__cta" aria-hidden="true">→</span>
            </button>
            <div v-if="view === 'standings'">
              <Leaderboard :entries="leaderboard" :my-id="myId || undefined" />
            </div>
            <ResultsBreakdown
              v-else
              :questions="results"
              :loading="resultsLoading"
              votable
              :my-vote="myVote"
              :voting="voting"
              @vote="handleVote"
            />
          </div>
        </template>
      </div>
      <div v-else class="card card--cream center stack">
        <div class="spinner" aria-hidden="true"></div>
        <span class="muted">Tallying scores…</span>
      </div>
    </transition>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted } from 'vue'
import { useGameStore } from '@/stores/game'
import { playerApi } from '@/services/api'
import Spotlight from '@/components/Spotlight.vue'
import Leaderboard from '@/components/Leaderboard.vue'
import ResultsBreakdown from '@/components/ResultsBreakdown.vue'
import type { QuestionResults } from '@/types'

const props = defineProps<{ code: string }>()
const store = useGameStore()
const phase = ref<'three' | 'two' | 'one' | 'ladder'>('three')
const initialReady = ref(false)
const view = ref<'standings' | 'breakdown'>('standings')
const results = ref<QuestionResults[]>([])
const resultsLoading = ref(false)
let resultsLoaded = false
const myVote = ref('')
const voting = ref(false)

const leaderboard = computed(() => store.leaderboard)
const byRank = computed(() => leaderboard.value)
const myId = computed(() => store.me && store.me.id)

async function setView(next: 'standings' | 'breakdown'): Promise<void> {
  view.value = next
  if (next === 'breakdown' && !resultsLoaded) {
    resultsLoading.value = true
    try {
      // Fetch the breakdown and the player's existing vote together so the UI
      // lands already locked if they voted on a previous visit.
      const [res, mine] = await Promise.all([
        playerApi.results(props.code),
        playerApi.myVote(props.code).catch(() => ({ questionId: '' })),
      ])
      results.value = res
      myVote.value = mine.questionId
      resultsLoaded = true
    } catch {
      // Leave results empty; the component shows an empty-state message.
    } finally {
      resultsLoading.value = false
    }
  }
}

async function handleVote(questionId: string): Promise<void> {
  if (myVote.value || voting.value) return
  voting.value = true
  try {
    const r = await playerApi.castVote(props.code, questionId)
    // Lock the player's pick. Counts are never shown here — only the admin sees
    // tallies — so there's nothing else to update.
    myVote.value = r.questionId
  } catch {
    // Swallow — the buttons re-enable so the player can retry.
  } finally {
    voting.value = false
  }
}

onMounted(async () => {
  await store.loadMe()
  store.ensureWS()
  try {
    store.setLeaderboard(await playerApi.leaderboard(props.code))
  } catch {}
  // Load the player's existing vote up front so the "vote for the best
  // question" hint only shows while they still have a vote to cast.
  playerApi.myVote(props.code).then(r => { myVote.value = r.questionId }).catch(() => {})
  if (leaderboard.value.length === 0) phase.value = 'ladder'
  else if (leaderboard.value.length === 1) phase.value = 'one'
  else if (leaderboard.value.length === 2) phase.value = 'two'

  await nextTick()
  initialReady.value = true
})
</script>

<style scoped>
.results-tabs {
  grid-template-columns: 1fr 1fr;
}

/* "Vote" badge on the Question breakdown tab. */
.breakdown-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.vote-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  background: var(--pink);
  color: var(--paper);
  font-size: .72rem;
  font-weight: 900;
  letter-spacing: .02em;
  line-height: 1.4;
  white-space: nowrap;
  transform-origin: center;
  animation: vote-badge-pop 1.4s ease-in-out infinite;
}
@keyframes vote-badge-pop {
  0%, 100% { transform: scale(1) rotate(-3deg); }
  50%      { transform: scale(1.12) rotate(3deg); }
}

/* Big attention-grabbing call to action on the standings view. */
.vote-callout {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  text-align: left;
  padding: 14px 16px;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r);
  background: var(--pink-2);
  color: var(--ink);
  cursor: pointer;
  box-shadow: 4px 4px 0 var(--ink);
  animation: vote-callout-bob 2.6s ease-in-out infinite;
  transition: transform .12s ease, box-shadow .12s ease;
}
.vote-callout:hover {
  transform: translate(-2px, -2px);
  box-shadow: 6px 6px 0 var(--ink);
  animation-play-state: paused;
}
.vote-callout:active {
  transform: translate(2px, 2px);
  box-shadow: 1px 1px 0 var(--ink);
}
@keyframes vote-callout-bob {
  0%, 100% { transform: translateY(0); }
  50%      { transform: translateY(-3px); }
}
.vote-callout__star {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: 2px solid var(--ink);
  background: var(--pink);
  color: var(--paper);
  box-shadow: 2px 2px 0 var(--ink);
  animation: vote-star-spin 3.4s ease-in-out infinite;
}
.vote-callout__star svg {
  width: 24px;
  height: 24px;
}
@keyframes vote-star-spin {
  0%, 70%, 100% { transform: rotate(0); }
  80%           { transform: rotate(-18deg); }
  90%           { transform: rotate(18deg); }
}
.vote-callout__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.vote-callout__title {
  font-family: var(--font-display, inherit);
  font-size: 1.05rem;
  line-height: 1.2;
}
.vote-callout__sub {
  font-size: .85rem;
  color: var(--ink);
  opacity: .8;
}
.vote-callout__cta {
  flex-shrink: 0;
  font-size: 1.5rem;
  font-weight: 900;
  line-height: 1;
}

@media (prefers-reduced-motion: reduce) {
  .vote-badge,
  .vote-callout,
  .vote-callout__star {
    animation: none;
  }
}
</style>
