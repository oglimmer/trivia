<template>
  <header class="header">
    <RouterLink to="/" class="logo">
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="9" />
        <path d="M9.5 9.5a2.5 2.5 0 1 1 4.3 1.7c-.9.8-1.8 1.4-1.8 2.6" />
        <circle cx="12" cy="17" r=".5" fill="currentColor" />
      </svg>
      Trivia
    </RouterLink>
    <span v-if="store.connected !== null" class="tag">
      {{ store.connected ? 'live' : 'offline' }}
    </span>
  </header>
  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>
</template>

<script setup>
import { RouterLink, RouterView } from 'vue-router'
import { useGameStore } from './stores/game.js'
const store = useGameStore()
</script>
