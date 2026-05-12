<template>
  <main class="stack">
    <!-- Phase: cinematic reveal of 3 -> 2 -> 1 then ladder -->
    <transition name="fade" mode="out-in">
      <div :key="phase" class="card stack" v-if="leaderboard.length">
        <template v-if="phase === 'three'">
          <h2 class="center">🥉 In third place…</h2>
          <Spotlight :score="byRank[2]" rank="3" color="bronze" />
          <button class="btn-primary" @click="phase = 'two'">Next →</button>
        </template>

        <template v-else-if="phase === 'two'">
          <h2 class="center">🥈 In second place…</h2>
          <Spotlight :score="byRank[1]" rank="2" color="silver" />
          <button class="btn-primary" @click="phase = 'one'">Next →</button>
        </template>

        <template v-else-if="phase === 'one'">
          <h2 class="center">🥇 The winner is…</h2>
          <Spotlight :score="byRank[0]" rank="1" color="gold" big />
          <button class="btn-accent" @click="phase = 'ladder'">Show full ladder</button>
        </template>

        <template v-else>
          <h2>Final standings</h2>
          <ol class="ladder">
            <li v-for="(s, i) in leaderboard" :key="s.userId" :class="{ me: s.userId === myId }">
              <span class="rank">{{ i + 1 }}</span>
              <img class="avatar" :src="s.photoB64" :alt="s.userName" />
              <span>{{ s.userName }}</span>
              <span class="pts">{{ s.points }} pts</span>
            </li>
          </ol>
          <RouterLink to="/" class="btn-ghost">← Back to start</RouterLink>
        </template>
      </div>
      <div v-else class="card center muted">Waiting for results…</div>
    </transition>
  </main>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useGameStore } from '../stores/game.js'
import { api } from '../services/api.js'
import Spotlight from '../components/Spotlight.vue'

const props = defineProps({ code: String })
const store = useGameStore()
const phase = ref('three')

const leaderboard = computed(() => store.leaderboard)
const byRank = computed(() => leaderboard.value)
const myId = computed(() => store.me && store.me.id)

onMounted(async () => {
  await store.loadMe()
  store.ensureWS()
  try {
    store.leaderboard = await api.leaderboard(props.code)
  } catch {}
  // If fewer than 3 players, skip ahead
  if (leaderboard.value.length < 3) phase.value = leaderboard.value.length === 0 ? 'ladder' : 'one'
})
</script>
