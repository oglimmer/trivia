<template>
  <ol class="ladder">
    <li v-for="(s, i) in entries" :key="s.userId" :class="{ me: myId && s.userId === myId }">
      <span class="rank">{{ i + 1 }}</span>
      <span v-if="online" class="avatar-wrap" :title="online.has(s.userId) ? 'Online' : 'Offline'">
        <img class="avatar" :src="imageUrl(s.photoImageId, 'thumb')" :alt="s.userName" loading="lazy" decoding="async" />
        <span class="presence-dot" :class="{ 'presence-dot--on': online.has(s.userId) }"></span>
      </span>
      <img v-else class="avatar" :src="imageUrl(s.photoImageId, 'thumb')" :alt="s.userName" loading="lazy" decoding="async" />
      <span class="bold">{{ s.userName }}</span>
      <span class="pts">{{ s.points }}</span>
    </li>
    <li v-if="!entries.length && emptyText" class="muted center" style="justify-content: center;">{{ emptyText }}</li>
  </ol>
</template>

<script setup lang="ts">
import { imageUrl } from '@/services/images'
import type { LeaderboardEntry } from '@/types'

defineProps<{
  entries: LeaderboardEntry[]
  myId?: string
  emptyText?: string
  online?: Set<string>
}>()
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
</style>
