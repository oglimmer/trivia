<template>
  <main class="stack-lg">
    <div class="card stack">
      <div class="row between">
        <h1 style="margin: 0;">Registered users</h1>
        <span class="tag tag--admin">Host</span>
      </div>
      <p class="muted" style="margin: -4px 0 0;">
        Every player ever registered, across all events.
      </p>

      <div class="row" style="gap: 10px; align-items: center;">
        <input
          v-model="filter"
          type="search"
          placeholder="Search by name…"
          aria-label="Filter users by name"
          style="flex: 1; min-width: 0;"
        />
        <span class="tag tag--blue">{{ filtered.length }} / {{ users.length }}</span>
        <RouterLink to="/admin/games" class="btn-ghost btn-sm">← All games</RouterLink>
      </div>

      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <div v-if="loading && !users.length" class="card card--cream center muted">
      <p style="margin: 0;">Loading…</p>
    </div>

    <div v-else-if="!filtered.length" class="card card--cream center muted">
      <p style="margin: 0;">{{ users.length ? 'No users match that filter.' : 'No registered users yet.' }}</p>
    </div>

    <div v-else class="user-grid">
      <div v-for="u in filtered" :key="u.id" class="card user-card">
        <button
          type="button"
          class="avatar-btn"
          @click="previewImage = u.photoB64 || ''"
          :aria-label="`Preview photo of ${u.name}`"
          :title="u.photoB64 ? 'Click to preview' : 'No photo'"
          :disabled="!u.photoB64"
        >
          <img v-if="u.photoB64" class="avatar avatar-lg" :src="u.photoB64" :alt="u.name" />
          <span v-else class="avatar avatar-lg avatar--placeholder" aria-hidden="true">{{ initials(u.name) }}</span>
        </button>
        <div class="user-card__meta">
          <div class="bold user-card__name">{{ u.name }}</div>
          <div class="muted user-card__game">
            <RouterLink :to="`/admin/games/${u.gameCode}`" class="mono">{{ u.gameCode }}</RouterLink>
            <span v-if="u.gameName"> · {{ u.gameName }}</span>
          </div>
          <div class="muted" style="font-size: .78rem;">Joined {{ formatDate(u.createdAt) }}</div>
        </div>
      </div>
    </div>

    <transition name="dialog">
      <div
        v-if="previewImage"
        class="img-preview-backdrop"
        role="dialog"
        aria-modal="true"
        aria-label="Photo preview"
        @mousedown.self="previewImage = ''"
        @keydown.esc.prevent="previewImage = ''"
        tabindex="-1"
      >
        <button
          type="button"
          class="img-preview-close"
          aria-label="Close preview"
          @click="previewImage = ''"
        >×</button>
        <img class="img-preview-img" :src="previewImage" alt="" />
      </div>
    </transition>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'
import type { AdminAllUser } from '@/types'

const users = ref<AdminAllUser[]>([])
const filter = ref('')
const loading = ref(false)
const err = ref('')
const previewImage = ref('')
const router = useRouter()
const store = useGameStore()

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(u =>
    u.name.toLowerCase().includes(q) ||
    u.gameCode.toLowerCase().includes(q) ||
    (u.gameName || '').toLowerCase().includes(q)
  )
})

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function formatDate(s: string): string {
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(async () => {
  if (!localStorage.getItem('adminToken')) { router.replace('/admin'); return }
  loading.value = true
  try {
    users.value = (await adminApi.listAllUsers()) || []
  } catch (e) {
    const msg = errMsg(e)
    if (msg.toLowerCase().includes('unauthorized')) {
      store.logoutAdmin(); router.replace('/admin'); return
    }
    err.value = msg
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.user-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}
.user-card {
  display: flex;
  gap: 12px;
  align-items: center;
}
.user-card__meta {
  min-width: 0;
  flex: 1;
}
.user-card__name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.user-card__game {
  font-size: .82rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.avatar-btn {
  padding: 0;
  border: 0;
  background: transparent;
  cursor: zoom-in;
  border-radius: 50%;
  flex-shrink: 0;
}
.avatar-btn[disabled] { cursor: default; }
.avatar-btn:focus-visible {
  outline: 3px solid var(--blue);
  outline-offset: 2px;
}
.avatar-lg {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  object-fit: cover;
  border: var(--bw) solid var(--ink);
  box-shadow: 2px 2px 0 var(--ink);
  background: var(--paper);
}
.avatar--placeholder {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-display);
  font-weight: 800;
  font-style: italic;
  color: var(--ink);
  background: var(--yellow);
}

.img-preview-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(26, 27, 38, .85);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 1100;
  cursor: zoom-out;
}
.img-preview-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-lg);
  background: var(--paper);
  box-shadow: var(--shadow-3);
  cursor: default;
}
.img-preview-close {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: var(--bw) solid var(--ink);
  background: var(--paper);
  color: var(--ink);
  font-size: 1.5rem;
  font-weight: 800;
  line-height: 1;
  cursor: pointer;
  box-shadow: var(--shadow-1);
}
.img-preview-close:hover { background: var(--coral); color: var(--paper); }
</style>
