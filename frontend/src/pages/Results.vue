<template>
  <main class="stack-lg">
    <transition name="fade" mode="out-in">
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
            <h1>Final standings</h1>
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
      </div>
      <div v-else class="card card--cream center stack">
        <div class="spinner" aria-hidden="true"></div>
        <span class="muted">Tallying scores…</span>
      </div>
    </transition>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useGameStore } from '@/stores/game'
import { playerApi } from '@/services/api'
import Spotlight from '@/components/Spotlight.vue'

const props = defineProps<{ code: string }>()
const store = useGameStore()
const phase = ref<'three' | 'two' | 'one' | 'ladder'>('three')

const leaderboard = computed(() => store.leaderboard)
const byRank = computed(() => leaderboard.value)
const myId = computed(() => store.me && store.me.id)

onMounted(async () => {
  await store.loadMe()
  store.ensureWS()
  try {
    store.setLeaderboard(await playerApi.leaderboard(props.code))
  } catch {}
  if (leaderboard.value.length < 3) phase.value = leaderboard.value.length === 0 ? 'ladder' : 'one'
})
</script>
