<template>
  <main class="stack-lg">
    <div class="card stack">
      <div class="row between" style="margin-bottom: 8px;">
        <h1 style="margin: 0;">Games</h1>
        <div class="row" style="gap: 8px; align-items: center;">
          <RouterLink to="/admin/users" class="btn-ghost btn-sm">All users →</RouterLink>
          <span class="tag tag--admin">Host</span>
        </div>
      </div>
      <p class="muted" style="margin: 0;">Spin up a room, share the code, let chaos begin.</p>

      <label for="game-name">Event name</label>
      <input id="game-name" v-model="name" placeholder="e.g. Family dinner — May 2025" />

      <label for="game-code">Code (optional)</label>
      <input id="game-code" v-model="code" placeholder="Random if blank" maxlength="8" class="mono" style="letter-spacing: .15em; text-transform: lowercase;" />

      <label for="game-scheduled">Scheduled date &amp; time (optional)</label>
      <input id="game-scheduled" v-model="scheduledAt" type="datetime-local" />

      <label for="game-timeout">Question timeout (seconds)</label>
      <div class="row">
        <input id="game-timeout" v-model.number="timeoutSeconds" type="number" min="5" max="600" step="1" />
        <button class="btn-primary" :disabled="loading" @click="create" style="flex-shrink: 0;">
          {{ loading ? '…' : 'Create' }}
        </button>
      </div>
      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <div v-if="games.length" class="stack">
      <div v-for="g in games" :key="g.id" class="card game-row">
        <div class="game-row__code">
          <span class="mono">{{ g.code }}</span>
        </div>
        <div class="game-row__meta">
          <div class="bold">{{ g.name || '(untitled)' }}</div>
          <div v-if="g.scheduledAt" class="muted" style="font-size: .85rem;">{{ formatScheduled(g.scheduledAt) }}</div>
          <div class="row" style="gap: 8px; font-size: .85rem; align-items: center;">
            <span :class="['state-pill', `state-${g.state}`]">{{ g.state }}</span>
            <span class="presence-pill" :class="{ 'presence-pill--on': (g.onlineCount || 0) > 0 }">
              <span class="presence-dot"></span>
              {{ g.onlineCount || 0 }} online
            </span>
          </div>
        </div>
        <div class="row" style="gap: 8px;">
          <RouterLink :to="`/admin/games/${g.code}`" class="btn-primary btn-sm">Open →</RouterLink>
          <button class="btn-danger btn-sm" :disabled="deleting === g.code" @click="remove(g)">
            {{ deleting === g.code ? '…' : 'Delete' }}
          </button>
        </div>
      </div>
    </div>
    <div v-else class="card card--cream center muted">
      <p style="margin: 0;">No games yet — create one above.</p>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'
import { confirm } from '@/services/dialog'
import type { AdminGamesEntry } from '@/types'

const games = ref<AdminGamesEntry[]>([])
const code = ref('')
const name = ref('')
const scheduledAt = ref('')
const timeoutSeconds = ref(30)
const loading = ref(false)
const deleting = ref('')
const err = ref('')
const router = useRouter()
const store = useGameStore()
let presencePoll: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  if (!localStorage.getItem('adminToken')) { router.replace('/admin'); return }
  await refresh()
  presencePoll = setInterval(refresh, 5000)
})

onUnmounted(() => {
  if (presencePoll) clearInterval(presencePoll)
})

async function refresh() {
  try {
    games.value = await adminApi.listGames() || []
  } catch (e) {
    const msg = errMsg(e)
    if (String(msg).toLowerCase().includes('unauthorized')) {
      store.logoutAdmin(); router.replace('/admin')
    } else err.value = msg
  }
}

async function create() {
  err.value = ''
  loading.value = true
  try {
    const g = await adminApi.createGame({
      code: code.value.trim().toLowerCase(),
      name: name.value,
      questionTimeoutSeconds: Number(timeoutSeconds.value) || 30,
      scheduledAt: scheduledAt.value ? new Date(scheduledAt.value).toISOString() : null,
    })
    code.value = ''
    name.value = ''
    scheduledAt.value = ''
    timeoutSeconds.value = 30
    router.push(`/admin/games/${g.code}`)
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    loading.value = false
  }
}

function formatScheduled(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

async function remove(g: AdminGamesEntry) {
  const label = g.name ? `"${g.name}" (${g.code})` : g.code
  const ok = await confirm({
    title: `Delete ${label}?`,
    message: 'This permanently removes the game, its players, questions, and answers. This cannot be undone.',
    confirmLabel: 'Delete',
    cancelLabel: 'Keep',
    tone: 'danger',
    icon: '🗑',
  })
  if (!ok) return
  err.value = ''
  deleting.value = g.code
  try {
    await adminApi.deleteGame(g.code)
    games.value = games.value.filter(x => x.id !== g.id)
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    deleting.value = ''
  }
}
</script>

<style scoped>
.game-row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 14px;
  align-items: center;
}
.game-row__code {
  font-family: var(--font-mono);
  font-weight: 800;
  font-size: 1.5rem;
  letter-spacing: .12em;
  padding: 10px 14px;
  background: var(--yellow);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
  text-transform: lowercase;
}
.state-pill {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  font-weight: 800;
  letter-spacing: .08em;
  text-transform: uppercase;
  font-size: .72rem;
  background: var(--paper);
  color: var(--ink);
}
.state-pill.state-setup    { background: var(--blue-2); }
.state-pill.state-game     { background: var(--pink); color: var(--paper); }
.state-pill.state-finished { background: var(--mint-2); }

.presence-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  font-weight: 700;
  letter-spacing: .04em;
  font-size: .7rem;
  background: var(--paper);
  color: var(--muted);
  text-transform: uppercase;
}
.presence-pill .presence-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted);
}
.presence-pill--on { color: var(--ink); }
.presence-pill--on .presence-dot {
  background: #22c55e;
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.25);
}

@media (max-width: 520px) {
  .game-row { grid-template-columns: 1fr; }
  .game-row__code { justify-self: flex-start; }
}
</style>
