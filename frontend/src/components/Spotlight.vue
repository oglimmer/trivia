<template>
  <div :class="['spot', color, big && 'big']" v-if="score">
    <div class="spot__rays" aria-hidden="true"></div>
    <div v-if="big" class="confetti" aria-hidden="true">
      <i v-for="n in 18" :key="n" :style="confettiStyle(n)"></i>
    </div>

    <span class="spot__rank">{{ rankLabel }}</span>
    <img class="avatar avatar-lg" :src="score.photoB64" :alt="score.userName" />
    <div class="spot__name">{{ score.userName }}</div>
    <div class="spot__pts">{{ score.points }} pts</div>
  </div>
  <div v-else class="muted center">—</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { LeaderboardEntry } from '@/types'

const props = withDefaults(defineProps<{
  score?: LeaderboardEntry | null
  rank: string | number
  color?: string
  big?: boolean
}>(), {
  score: null,
  color: 'gold',
  big: false,
})

const rankLabel = computed(() => {
  const map: Record<string, string> = { '1': '🥇 First', '2': '🥈 Second', '3': '🥉 Third' }
  return map[String(props.rank)] || `#${props.rank}`
})

function confettiStyle(n: number) {
  const left = (n * 17) % 100
  const delay = (n * 0.13) % 2
  const dur = 2.2 + (n % 5) * 0.3
  return {
    left: `${left}%`,
    animationDelay: `${delay}s`,
    animationDuration: `${dur}s`,
    transform: `rotate(${n * 28}deg)`,
  }
}
</script>
