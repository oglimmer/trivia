<template>
  <main class="stack">
    <div class="card stack">
      <h1>Games</h1>

      <div class="row">
        <input v-model="name" placeholder="Event name (optional)" />
      </div>
      <div class="row">
        <input v-model="code" placeholder="Code (blank = random)" maxlength="8" />
        <button class="btn-primary" :disabled="loading" @click="create">
          {{ loading ? '…' : 'Create' }}
        </button>
      </div>
      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <div class="stack">
      <div v-for="g in games" :key="g.id" class="card row between">
        <div>
          <div style="font-size: 1.2rem; font-weight: 600;">{{ g.code }}</div>
          <div class="muted" style="font-size: .85rem;">{{ g.name || '(no name)' }} · {{ g.state }}</div>
        </div>
        <div class="row" style="gap: 8px;">
          <RouterLink :to="`/admin/games/${g.code}`" class="btn-primary">Open</RouterLink>
          <button class="btn-danger" :disabled="deleting === g.code" @click="remove(g)">
            {{ deleting === g.code ? '…' : 'Delete' }}
          </button>
        </div>
      </div>
      <div v-if="games.length === 0" class="card center muted">No games yet.</div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../services/api.js'
import { useGameStore } from '../stores/game.js'
import { confirm } from '../services/dialog.js'

const games = ref([])
const code = ref('')
const name = ref('')
const loading = ref(false)
const deleting = ref('')
const err = ref('')
const router = useRouter()
const store = useGameStore()

onMounted(async () => {
  if (!localStorage.getItem('adminToken')) { router.replace('/admin'); return }
  await refresh()
})

async function refresh() {
  try {
    games.value = await api.adminGames() || []
  } catch (e) {
    if (String(e.message).toLowerCase().includes('unauthorized')) {
      store.logoutAdmin(); router.replace('/admin')
    } else err.value = e.message
  }
}

async function create() {
  err.value = ''
  loading.value = true
  try {
    const g = await api.adminCreateGame({ code: code.value.trim().toLowerCase(), name: name.value })
    code.value = ''
    name.value = ''
    router.push(`/admin/games/${g.code}`)
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

async function remove(g) {
  const label = g.name ? `"${g.name}" (${g.code})` : g.code
  const ok = await confirm({
    title: `Delete ${label}?`,
    message: 'This permanently removes the game, its players, questions, and answers. This action cannot be undone.',
    confirmLabel: 'Delete',
    cancelLabel: 'Keep',
    tone: 'danger',
    icon: '🗑️',
  })
  if (!ok) return
  err.value = ''
  deleting.value = g.code
  try {
    await api.adminDeleteGame(g.code)
    games.value = games.value.filter(x => x.id !== g.id)
  } catch (e) {
    err.value = e.message
  } finally {
    deleting.value = ''
  }
}

</script>
