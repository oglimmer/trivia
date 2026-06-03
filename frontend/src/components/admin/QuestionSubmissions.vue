<template>
  <div class="card card--cream">
    <div class="row between" style="margin-bottom: 12px;">
      <h2 style="margin: 0;">Submissions</h2>
      <span class="tag tag--yellow">{{ questions.length }}</span>
    </div>
    <div class="stack">
      <div v-for="q in questions" :key="q.id" class="card card--flat" style="background: var(--paper); border: 2px solid var(--ink); padding: 14px;">
        <div class="row" style="gap: 12px; align-items: flex-start;">
          <button
            type="button"
            class="avatar-btn"
            @click="emit('preview', imageUrl(q.photoImageId, 'orig'))"
            :aria-label="`Preview photo by ${userName(q.userId)}`"
            title="Click to preview"
          >
            <img class="avatar" :src="imageUrl(q.photoImageId, 'thumb')" alt="" loading="lazy" decoding="async" />
          </button>
          <div style="flex: 1; min-width: 0;">
            <div class="bold">{{ q.text }}</div>
            <div class="muted" style="font-size: .85rem;">
              <span class="kbd" style="padding: 1px 6px; font-size: .75rem;">{{ q.answerType }}</span>
              · by {{ userName(q.userId) }}
            </div>
          </div>
          <button
            class="btn-danger btn-sm btn-icon-sm"
            :disabled="deletingId === q.id"
            @click="emit('delete', q)"
            :aria-label="`Delete submission by ${userName(q.userId)}`"
            title="Delete submission"
          >
            <span v-if="deletingId === q.id">…</span>
            <svg v-else class="trash-icon" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 6h18" />
              <path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
              <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
              <path d="M10 11v6" />
              <path d="M14 11v6" />
            </svg>
          </button>
        </div>
        <details style="margin-top: 10px;">
          <summary class="bold" style="cursor: pointer;">Reveal answer</summary>
          <div v-if="q.answerType === 'choice'" class="stack" style="margin-top: 8px;">
            <div
              v-for="(o, i) in q.options"
              :key="i"
              :class="['option-btn', i === Number(q.correct) && 'correct']"
            >
              <span class="option-btn__bullet">{{ letters[i] }}</span>{{ o }}
            </div>
          </div>
          <div v-else class="card card--mint" style="padding: 12px; margin-top: 8px; text-align: center;">
            <span class="bold" style="font-family: var(--font-display); font-style: italic; font-size: 1.4rem;">{{ q.correct }}</span>
          </div>
        </details>
      </div>
      <div v-if="questions.length === 0" class="muted center">No submissions yet.</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { imageUrl } from '@/services/images'
import type { Question, User } from '@/types'

const props = defineProps<{
  questions: Question[]
  users: User[]
  deletingId: string
}>()

const emit = defineEmits<{
  (e: 'preview', src: string): void
  (e: 'delete', q: Question): void
}>()

const letters = ['A', 'B', 'C', 'D']

const usersById = computed(() => Object.fromEntries(props.users.map(u => [u.id, u])))
function userName(id: string): string {
  return usersById.value[id]?.name || '...'
}
</script>

<style scoped>
.avatar-btn {
  padding: 0;
  border: 0;
  background: transparent;
  cursor: zoom-in;
  border-radius: 50%;
  flex-shrink: 0;
}
.avatar-btn:focus-visible {
  outline: 3px solid var(--blue);
  outline-offset: 2px;
}
.trash-icon {
  width: 18px;
  height: 18px;
}
</style>
