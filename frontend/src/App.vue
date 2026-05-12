<template>
  <header class="app-header">
    <div class="app-header__inner">
      <RouterLink to="/" class="brand" aria-label="Trivia home">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
          <path d="M9.5 9.5a2.5 2.5 0 1 1 4.3 1.7c-.9.8-1.8 1.4-1.8 2.6" />
          <circle cx="12" cy="17" r=".5" fill="currentColor" />
        </svg>
        <span>Trivia</span>
      </RouterLink>

      <nav class="app-header__nav">
        <span v-if="store.connected !== null" :class="['conn', store.connected ? 'conn--live' : 'conn--off']" :title="store.connected ? 'Connected' : 'Reconnecting…'">
          <span class="conn__dot" aria-hidden="true"></span>
          {{ store.connected ? 'live' : 'offline' }}
        </span>

        <template v-if="store.me">
          <div class="who" :title="store.me.name">
            <img v-if="store.me.photoB64" class="avatar avatar-sm" :src="store.me.photoB64" alt="" />
            <div class="who__meta">
              <span class="who__name">{{ store.me.name }}</span>
              <span v-if="store.game" class="who__code">{{ store.game.code }}</span>
            </div>
          </div>
          <button class="btn-ghost btn-sm" @click="leave">Leave</button>
        </template>

        <template v-else-if="store.isAdmin">
          <span class="tag tag--admin">Admin</span>
          <button class="btn-ghost btn-sm" @click="signOutAdmin">Sign out</button>
        </template>
      </nav>
    </div>
  </header>
  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>
  <ConfirmDialog />
</template>

<script setup>
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useGameStore } from './stores/game.js'
import { disconnect } from './services/ws.js'
import { confirm } from './services/dialog.js'
import ConfirmDialog from './components/ConfirmDialog.vue'

const store = useGameStore()
const router = useRouter()

async function leave() {
  const ok = await confirm({
    title: 'Leave this game?',
    message: 'You can rejoin later with the same code.',
    confirmLabel: 'Leave game',
    cancelLabel: 'Stay',
    tone: 'danger',
    icon: '👋',
  })
  if (!ok) return
  disconnect()
  store.logout()
  router.push('/')
}

function signOutAdmin() {
  store.logoutAdmin()
  router.push('/admin')
}
</script>
