<template>
  <header class="app-header">
    <div class="app-header__inner">
      <RouterLink to="/" class="brand" aria-label="Trivia home">
        <span class="brand__mark" aria-hidden="true">T</span>
        <span class="brand__name">Trivia<em class="brand__dot">.</em></span>
      </RouterLink>

      <nav class="app-header__nav">
        <span
          v-if="store.connected !== null"
          :class="['conn', store.connected ? 'conn--live' : 'conn--off']"
          :title="store.connected ? 'Connected' : 'Reconnecting…'"
        >
          <span class="conn__dot" aria-hidden="true"></span>
          {{ store.connected ? 'Live' : 'Off' }}
        </span>

        <template v-if="store.me">
          <button
            type="button"
            class="who"
            :title="`Edit profile · ${store.me.name}`"
            aria-label="Edit profile"
            @click="editing = true"
          >
            <img v-if="store.me.photoB64" class="avatar avatar-sm" :src="store.me.photoB64" alt="" />
            <div class="who__meta">
              <span class="who__name">{{ store.me.name }}</span>
              <span v-if="store.game" class="who__code">{{ store.game.code }}</span>
            </div>
          </button>
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

  <footer class="foot">made for game night ★ no big tech, just friends</footer>

  <ConfirmDialog />
  <ProfileDialog :open="editing" @close="editing = false" />
</template>

<script setup>
import { ref } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { useGameStore } from './stores/game.js'
import { disconnect } from './services/ws.js'
import { confirm } from './services/dialog.js'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ProfileDialog from './components/ProfileDialog.vue'

const store = useGameStore()
const router = useRouter()
const editing = ref(false)

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

<style scoped>
.brand__name { line-height: 1; }
.brand__dot {
  font-style: italic;
  color: var(--pink);
}
</style>
