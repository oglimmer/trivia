<template>
  <div class="stack">
    <label for="import-json" class="bold">Paste JSON</label>
    <textarea
      id="import-json"
      v-model="raw"
      rows="10"
      spellcheck="false"
      class="mono import-textarea"
      :placeholder="placeholder"
    ></textarea>

    <div class="row wrap" style="gap: 10px; align-items: center;">
      <button class="btn-primary" :disabled="!parsed || busy" @click="submit">
        {{ busy ? 'Importing…' : (parsed ? `Replace all with ${parsed.questions.length} questions` : 'Import') }}
      </button>
      <button class="btn-ghost btn-sm" @click="raw = sample">Insert example</button>
      <span v-if="parsed" class="muted" style="font-size: .85rem;">
        {{ parsed.questions.length }} questions · {{ parsed.questions.length * 5 }} answers · looks valid
      </span>
    </div>

    <div v-if="parseError" class="error">{{ parseError }}</div>
    <div v-if="importError" class="error">{{ importError }}</div>
    <div v-if="done" class="card card--mint" style="padding: 10px 14px;">
      Imported {{ done }} questions. Answer order was shuffled so the top answer isn't always first.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { adminApi } from '@/services/api'
import { errMsg } from '@/composables/errMsg'
import type { ImportQuestionsBody } from '@/services/api'


const props = defineProps<{ code: string }>()
const emit = defineEmits<{ (e: 'imported'): void }>()

const raw = ref('')
const busy = ref(false)
const importError = ref('')
const done = ref(0)

const sample = JSON.stringify({
  questions: [
    {
      text: 'Name something you always forget to pack.',
      answers: [
        { text: 'Toothbrush', points: 41 },
        { text: 'Phone charger', points: 22 },
        { text: 'Socks', points: 11 },
        { text: 'Sunscreen', points: 7 },
        { text: 'Passport', points: 4 },
      ],
    },
  ],
}, null, 2)

const placeholder = '{\n  "questions": [\n    {\n      "text": "…",\n      "answers": [{ "text": "…", "points": 41 }, … 5 total]\n    }\n  ]\n}'

// Validated client-side too, so a typo is caught before a round trip. The
// backend re-validates — this is convenience, not the guard.
interface ParseResult { body: ImportQuestionsBody | null; error: string }

function parseImport(text: string): ParseResult {
  const trimmed = text.trim()
  if (!trimmed) return { body: null, error: '' }
  let obj: unknown
  try {
    obj = JSON.parse(trimmed)
  } catch (e) {
    return { body: null, error: 'Not valid JSON: ' + (e instanceof Error ? e.message : String(e)) }
  }
  const body = obj as ImportQuestionsBody
  if (!body || !Array.isArray(body.questions) || !body.questions.length) {
    return { body: null, error: 'Expected an object with a non-empty "questions" array.' }
  }
  for (let i = 0; i < body.questions.length; i++) {
    const q = body.questions[i]
    const where = `Question ${i + 1}`
    if (!q || typeof q.text !== 'string' || !q.text.trim()) {
      return { body: null, error: `${where}: "text" is required.` }
    }
    if (!Array.isArray(q.answers) || q.answers.length !== 5) {
      const found = Array.isArray(q.answers) ? q.answers.length : 0
      return { body: null, error: `${where}: needs exactly 5 answers, found ${found}.` }
    }
    for (let j = 0; j < q.answers.length; j++) {
      const a = q.answers[j]
      if (!a || typeof a.text !== 'string' || !a.text.trim()) {
        return { body: null, error: `${where}, answer ${j + 1}: "text" is required.` }
      }
      if (typeof a.points !== 'number' || !isFinite(a.points) || a.points < 0) {
        return { body: null, error: `${where}, answer ${j + 1}: "points" must be a number of 0 or more.` }
      }
    }
  }
  return { body, error: '' }
}

const parseResult = computed(() => parseImport(raw.value))
const parsed = computed(() => parseResult.value.body)
const parseError = computed(() => parseResult.value.error)

async function submit() {
  const body = parsed.value
  if (!body) return
  importError.value = ''
  done.value = 0
  busy.value = true
  try {
    const res = await adminApi.importQuestions(props.code, body)
    done.value = res.imported
    raw.value = ''
    emit('imported')
  } catch (e) {
    importError.value = errMsg(e, 'Could not import')
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.import-textarea { width: 100%; font-size: .85rem; line-height: 1.45; }
</style>
