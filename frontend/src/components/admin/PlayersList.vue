<template>
  <div class="card">
    <div class="row between" style="margin-bottom: 12px;">
      <h2 style="margin: 0;">Players</h2>
      <span class="tag tag--blue">{{ onlineCount }} / {{ users.length }} online</span>
    </div>
    <ul class="ladder">
      <li v-for="u in users" :key="u.id">
        <span class="avatar-wrap">
          <img class="avatar" :src="imageUrl(u.photoImageId, 'thumb')" :alt="u.name" loading="lazy" decoding="async" />
          <span class="presence-dot" :class="{ 'presence-dot--on': online.has(u.id) }" :title="online.has(u.id) ? 'Online' : 'Offline'"></span>
        </span>
        <span class="bold" :class="{ 'muted': !online.has(u.id) }">{{ u.name }}</span>
        <span class="pts" :style="`color: ${readyIds.has(u.id) ? 'var(--mint)' : 'var(--muted)'};`">
          {{ readyIds.has(u.id) ? '✓ ready' : 'thinking…' }}
        </span>
        <button
          class="btn-ghost btn-sm btn-icon-sm"
          :disabled="copyingId === u.id"
          @click="emit('copy', u)"
          style="margin-left: auto;"
          :aria-label="`Copy login link for ${u.name}`"
          :title="`Copy a link that signs you in as ${u.name}`"
        >
          <span v-if="copyingId === u.id">…</span>
          <span v-else-if="copiedId === u.id" aria-hidden="true">✓</span>
          <span v-else aria-hidden="true">🔗</span>
        </button>
        <button
          class="btn-danger btn-sm btn-icon-sm"
          :disabled="deletingId === u.id"
          @click="emit('delete', u)"
          :aria-label="`Remove ${u.name}`"
          title="Remove player"
        >
          <span v-if="deletingId === u.id">…</span>
          <span v-else aria-hidden="true">🗑</span>
        </button>
      </li>
      <li v-if="!users.length" class="muted center" style="justify-content: center;">No players yet.</li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { imageUrl } from '@/services/images'
import type { User } from '@/types'

const props = defineProps<{
  users: User[]
  online: Set<string>
  readyIds: Set<string>
  copyingId: string
  copiedId: string
  deletingId: string
}>()

const emit = defineEmits<{
  (e: 'copy', user: User): void
  (e: 'delete', user: User): void
}>()

const onlineCount = computed(() => props.users.filter(u => props.online.has(u.id)).length)
</script>

<style scoped>
.avatar-wrap {
  position: relative;
  display: inline-flex;
}
.presence-dot {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #9ca3af;
  border: 2px solid var(--paper);
  box-sizing: border-box;
}
.presence-dot--on { background: #22c55e; }

.btn-icon-sm {
  padding: 0;
  width: 36px;
  height: 36px;
  min-width: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  line-height: 1;
  flex-shrink: 0;
}
</style>
