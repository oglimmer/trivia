<template>
  <div class="card stack center" :class="['spot', color, big && 'big']" v-if="score">
    <div class="halo" aria-hidden="true"></div>
    <img class="avatar avatar-lg" :src="score.photoB64" :alt="score.userName" />
    <div style="font-size: 1.6rem; font-weight: 700;">{{ score.userName }}</div>
    <div class="muted">{{ score.points }} points</div>
  </div>
  <div v-else class="muted center">—</div>
</template>

<script setup>
defineProps({
  score: { type: Object, default: null },
  rank: { type: [String, Number], required: true },
  color: { type: String, default: 'gold' },
  big: { type: Boolean, default: false },
})
</script>

<style scoped>
.spot { position: relative; overflow: hidden; }
.halo {
  position: absolute; inset: -20%; pointer-events: none;
  background: radial-gradient(circle at 50% 30%, var(--c, gold) 0%, transparent 55%);
  opacity: .25;
}
.spot.gold   { --c: #ffd166; }
.spot.silver { --c: #d8dee9; }
.spot.bronze { --c: #cd7f32; }
.spot.big .avatar-lg { width: 128px; height: 128px; }
</style>
