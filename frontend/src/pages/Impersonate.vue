<template>
  <main class="stack-lg">
    <div class="card stack center">
      <h2 v-if="!err">Signing you in…</h2>
      <h2 v-else>Could not sign in</h2>
      <div v-if="err" class="error">{{ err }}</div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { playerApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'
import { disconnect } from '@/services/ws'

const router = useRouter()
const store = useGameStore()
const err = ref('')

function parseHash() {
  const h = window.location.hash || ''
  const q = h.startsWith('#') ? h.slice(1) : h
  return new URLSearchParams(q)
}

onMounted(async () => {
  const p = parseHash()
  const token = p.get('token')
  if (!token) {
    err.value = 'Missing token in link.'
    return
  }
  disconnect()
  store.logoutAdmin()
  store.logout()

  try {
    localStorage.setItem('playerToken', token)
    const r = await playerApi.me()
    store.setMe(token, r.user)
    store.setGame(r.game)
    history.replaceState(null, '', '/')
    const code = r.game?.code
    const state = r.game?.state
    if (!code) { router.replace('/'); return }
    if (state === 'setup') router.replace(`/g/${code}/setup`)
    else if (state === 'game') router.replace(`/g/${code}/play`)
    else if (state === 'finished') router.replace(`/g/${code}/results`)
    else router.replace('/')
  } catch (e) {
    localStorage.removeItem('playerToken')
    err.value = errMsg(e, 'Invalid or expired token.')
  }
})
</script>
