<template>
  <main class="stack-lg" style="padding-top: 12px;">
    <section class="hero notfound-hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__sparkle s3" aria-hidden="true">★</span>

      <span class="hero__eyebrow">Lost in space</span>
      <div class="notfound-code" aria-hidden="true">
        <span class="notfound-digit">4</span>
        <span class="notfound-digit notfound-digit--ball">0</span>
        <span class="notfound-digit">4</span>
      </div>
      <h1 class="hero__title">Page <em>not</em> found.</h1>
      <p class="hero__subtitle">
        We couldn't find <code class="notfound-path">{{ path }}</code>.<br />
        Maybe a typo in the link, or it moved on without telling us.
      </p>
    </section>

    <section class="card stack center">
      <p class="muted" style="margin: 0;">Try one of these instead:</p>
      <div class="row" style="justify-content: center; gap: 12px; flex-wrap: wrap;">
        <RouterLink to="/" class="btn btn-primary btn-lg">← Back to home</RouterLink>
        <RouterLink to="/admin" class="btn btn-ghost">Open admin</RouterLink>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

const route = useRoute()
const path = computed(() => route.fullPath || '/')
</script>

<style scoped>
.notfound-hero {
  background: var(--pink-2);
}
.notfound-code {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 10px;
  margin: 6px 0 14px;
  font-family: var(--font-display);
  font-style: italic;
  font-weight: 900;
  line-height: 1;
}
.notfound-digit {
  font-size: clamp(4rem, 18vw, 7rem);
  color: var(--ink);
  background: var(--paper);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-2);
  padding: 6px 18px 10px;
  transform: rotate(-4deg);
}
.notfound-digit:nth-child(2) { transform: rotate(3deg); }
.notfound-digit:nth-child(3) { transform: rotate(-2deg); }
.notfound-digit--ball {
  background: var(--yellow);
  border-radius: 50%;
  padding: 6px 22px 10px;
  animation: notfound-bob 2.4s ease-in-out infinite;
}
@keyframes notfound-bob {
  0%, 100% { transform: rotate(3deg) translateY(0); }
  50%      { transform: rotate(3deg) translateY(-6px); }
}
@media (prefers-reduced-motion: reduce) {
  .notfound-digit--ball { animation: none; }
}
.notfound-path {
  font-family: var(--font-mono);
  font-weight: 700;
  background: var(--paper);
  border: 2px solid var(--ink);
  border-radius: 8px;
  padding: 1px 8px;
  word-break: break-all;
}
</style>
