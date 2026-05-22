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
                :class="{ active: view === 'breakdown' }"
                @click="setView('breakdown')"
              >Question breakdown</button>
            </div>
            <div v-if="view === 'standings'">
              <Leaderboard :entries="leaderboard" :my-id="myId || undefined" />
            </div>
            <ResultsBreakdown
              v-else
              :questions="results"
              :loading="resultsLoading"
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

const leaderboard = computed(() => store.leaderboard)
const byRank = computed(() => leaderboard.value)
const myId = computed(() => store.me && store.me.id)

async function setView(next: 'standings' | 'breakdown'): Promise<void> {
  view.value = next
  if (next === 'breakdown' && !resultsLoaded) {
    resultsLoading.value = true
    try {
      results.value = await playerApi.results(props.code)
      resultsLoaded = true
    } catch {
      // Leave results empty; the component shows an empty-state message.
    } finally {
      resultsLoading.value = false
    }
  }
}

onMounted(async () => {
  await store.loadMe()
  store.ensureWS()
  try {
    store.setLeaderboard(await playerApi.leaderboard(props.code))
  } catch {}
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
</style>
