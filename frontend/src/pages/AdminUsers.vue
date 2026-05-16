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
          @click="previewImage = imageUrl(u.photoImageId, 'orig')"
          :aria-label="`Preview photo of ${u.name}`"
          :title="hasPhoto(u) ? 'Click to preview' : 'No photo'"
          :disabled="!hasPhoto(u)"
        >
          <img
            v-if="hasPhoto(u)"
            class="avatar avatar-lg"
            :src="imageUrl(u.photoImageId, 'thumb')"
            :alt="u.name"
            loading="lazy"
            decoding="async"
          />
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

    <ImagePreviewModal :src="previewImage" @close="previewImage = ''" />
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '@/services/api'
import { imageUrl } from '@/services/images'
import { errMsg } from '@/composables/errMsg'
import ImagePreviewModal from '@/components/ImagePreviewModal.vue'
import type { AdminAllUser } from '@/types'

const users = ref<AdminAllUser[]>([])
const filter = ref('')
const loading = ref(false)
const err = ref('')
const previewImage = ref('')
const router = useRouter()

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(u =>
    u.name.toLowerCase().includes(q) ||
    u.gameCode.toLowerCase().includes(q) ||
    (u.gameName || '').toLowerCase().includes(q)
  )
})

function hasPhoto(u: AdminAllUser): boolean {
  return !!u.photoImageId
}

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
    err.value = errMsg(e)
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

</style>
