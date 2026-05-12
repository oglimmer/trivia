<template>
  <main class="stack" style="padding-top: 32px;">
    <div class="card stack center">
      <div style="font-size: 3rem; line-height: 1;">🎯</div>
      <h1>Join a game</h1>
      <p class="muted">Enter the 4-character code from your host.</p>
      <input
        v-model="code"
        @keyup.enter="join"
        placeholder="e.g. id5x"
        maxlength="8"
        autocapitalize="off"
        autocomplete="off"
        spellcheck="false"
        style="font-size: 1.4rem; letter-spacing: .2em; text-align: center; text-transform: lowercase;"
      />
      <button class="btn-primary" :disabled="!code || loading" @click="join">
        {{ loading ? 'Looking up…' : 'Continue' }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <RouterLink to="/admin" class="card row between" style="text-decoration: none; color: inherit;">
      <span>
        <strong>Admin</strong>
        <div class="muted" style="font-size: .85rem;">Create &amp; control games</div>
      </span>
      <span aria-hidden="true">→</span>
    </RouterLink>
  </main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../services/api.js'
import { useGameStore } from '../stores/game.js'

const router = useRouter()
const code = ref('')
const loading = ref(false)
const err = ref('')
const store = useGameStore()

onMounted(async () => {
  await store.loadMe()
  if (store.me && store.game) {
    routeForState(store.game.code, store.game.state)
  }
})

function routeForState(c, state) {
  if (state === 'setup') router.replace(`/g/${c}/setup`)
  else if (state === 'game') router.replace(`/g/${c}/play`)
  else if (state === 'finished') router.replace(`/g/${c}/results`)
}

async function join() {
  err.value = ''
  loading.value = true
  try {
    const c = code.value.trim().toLowerCase()
    const g = await api.getGame(c)
    if (g.state === 'setup') router.push(`/g/${c}/join`)
    else router.push(`/g/${c}/play`)
  } catch (e) {
    err.value = e.message || 'No game with that code'
  } finally {
    loading.value = false
  }
}
</script>
