<template>
  <div class="card stack">
    <div class="row between" style="align-items: flex-start;">
      <div>
        <h2 style="margin: 0;">Questions</h2>
        <p class="muted" style="margin: 4px 0 0; font-size: .85rem;">
          Played in this order. Teams see the answers shuffled.
        </p>
      </div>
      <span class="tag tag--yellow">{{ questions.length }}</span>
    </div>

    <ol v-if="questions.length" class="pq-list">
      <li v-for="(q, i) in questions" :key="q.id" class="pq-item">
        <template v-if="editingId === q.id">
          <PollQuestionForm
            :code="code"
            :question="q"
            @saved="onSaved"
            @cancel="editingId = ''"
          />
        </template>
        <template v-else>
          <div class="pq-item__head">
            <span class="pq-item__num">{{ i + 1 }}</span>
            <div class="pq-item__text">{{ q.text }}</div>
            <div class="pq-item__actions">
              <button
                class="btn-ghost btn-sm btn-icon-sm"
                :disabled="i === 0 || !!movingId"
                :aria-label="`Move question ${i + 1} up`"
                title="Move up"
                @click="move(q, 'up')"
              >↑</button>
              <button
                class="btn-ghost btn-sm btn-icon-sm"
                :disabled="i === questions.length - 1 || !!movingId"
                :aria-label="`Move question ${i + 1} down`"
                title="Move down"
                @click="move(q, 'down')"
              >↓</button>
              <button
                class="btn-ghost btn-sm"
                :aria-label="`Edit question ${i + 1}`"
                @click="startEdit(q)"
              >Edit</button>
              <button
                class="btn-danger btn-sm"
                :disabled="deletingId === q.id"
                :aria-label="`Remove question ${i + 1}`"
                @click="remove(q)"
              >{{ deletingId === q.id ? '…' : 'Remove' }}</button>
            </div>
          </div>
          <ul class="pq-item__answers">
            <li v-for="(o, oi) in rankedOptions(q)" :key="oi">
              <span class="pq-item__rank">{{ oi + 1 }}</span>
              <span class="pq-item__answer">{{ o.text }}</span>
              <span class="pq-item__pts">{{ o.points }}</span>
            </li>
          </ul>
        </template>
      </li>
    </ol>

    <div v-else class="card card--cream center muted" style="padding: 18px;">
      <p style="margin: 0;">No questions yet — add your first one below.</p>
    </div>

    <div v-if="err" class="error">{{ err }}</div>

    <PollQuestionForm
      v-if="adding"
      :code="code"
      @saved="onSaved"
      @cancel="adding = false"
    />
    <button v-else class="btn-primary" style="align-self: flex-start;" @click="startAdd">
      + Add question
    </button>

    <details class="pq-bulk">
      <summary>Bulk import from JSON</summary>
      <p class="muted" style="font-size: .85rem;">
        Replaces every question above in one go. Useful for pasting a whole
        survey at once; everything stays editable afterwards.
      </p>
      <QuestionImport :code="code" @imported="onSaved" />
    </details>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import PollQuestionForm from './PollQuestionForm.vue'
import QuestionImport from './QuestionImport.vue'
import { adminApi } from '@/services/api'
import { errMsg } from '@/composables/errMsg'
import { confirm } from '@/services/dialog'
import type { PollOption, Question } from '@/types'

const props = defineProps<{ code: string; questions: Question[] }>()
const emit = defineEmits<{ (e: 'changed'): void }>()

const adding = ref(false)
const editingId = ref('')
const deletingId = ref('')
const movingId = ref('')
const err = ref('')

// Stored order is shuffled; rank by points for the host's view so the list
// reads like the survey results it came from.
function rankedOptions(q: Question): PollOption[] {
  return [...((q.options || []) as PollOption[])]
    .map(o => ({ text: o.text, points: o.points ?? 0 }))
    .sort((a, b) => b.points - a.points)
}

function startAdd() {
  editingId.value = ''
  adding.value = true
}

function startEdit(q: Question) {
  adding.value = false
  editingId.value = q.id
}

function onSaved() {
  editingId.value = ''
  emit('changed')
}

async function move(q: Question, direction: 'up' | 'down') {
  err.value = ''
  movingId.value = q.id
  try {
    await adminApi.moveQuestion(props.code, q.id, direction)
    emit('changed')
  } catch (e) {
    err.value = errMsg(e, 'Could not reorder')
  } finally {
    movingId.value = ''
  }
}

async function remove(q: Question) {
  const ok = await confirm({
    title: 'Remove this question?',
    message: q.text,
    confirmLabel: 'Remove',
    tone: 'danger',
  })
  if (!ok) return
  err.value = ''
  deletingId.value = q.id
  try {
    await adminApi.deleteQuestion(props.code, q.id)
    emit('changed')
  } catch (e) {
    err.value = errMsg(e, 'Could not remove')
  } finally {
    deletingId.value = ''
  }
}
</script>

<style scoped>
.pq-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 12px; }
.pq-item {
  border: 2px solid var(--ink);
  background: var(--paper);
  padding: 12px;
}
.pq-item__head {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 10px;
  align-items: start;
}
.pq-item__num { font-weight: 900; opacity: .5; min-width: 1.4em; }
.pq-item__text { font-weight: 700; min-width: 0; }
.pq-item__actions { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-end; }
.pq-item__answers {
  list-style: none;
  margin: 10px 0 0 calc(1.4em + 10px);
  padding: 0;
  display: grid;
  gap: 2px;
  font-size: .85rem;
  max-width: 420px;
}
.pq-item__answers li { display: grid; grid-template-columns: auto 1fr auto; gap: 8px; }
.pq-item__rank { opacity: .45; font-variant-numeric: tabular-nums; }
.pq-item__answer { min-width: 0; }
.pq-item__pts { font-variant-numeric: tabular-nums; font-weight: 800; }
.pq-bulk summary { cursor: pointer; font-weight: 700; }
</style>
