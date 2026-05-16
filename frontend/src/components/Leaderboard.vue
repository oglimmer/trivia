<template>
  <ol class="ladder">
    <li v-for="(s, i) in entries" :key="s.userId" :class="{ me: myId && s.userId === myId }">
      <span class="rank">{{ i + 1 }}</span>
      <img class="avatar" :src="imageUrl(s.photoImageId, 'thumb')" :alt="s.userName" loading="lazy" decoding="async" />
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
}>()
</script>
