<template>
  <transition name="dialog">
    <div
      v-if="open"
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      aria-labelledby="build-dlg-title"
      @mousedown.self="close"
      @keydown.esc.prevent="close"
    >
      <div class="modal stack build-dlg">
        <div class="dialog__icon" aria-hidden="true">ℹ</div>
        <h2 id="build-dlg-title" class="dialog__title">Build info</h2>

        <dl class="build-dlg__grid">
          <dt>Frontend</dt>
          <dd>
            <span class="build-dlg__ver">v{{ frontend.version }}</span>
            <span class="build-dlg__meta">{{ frontend.gitCommit }} · {{ formatBuildTime(frontend.buildTime) }}</span>
          </dd>

          <dt>Backend</dt>
          <dd v-if="backend">
            <span class="build-dlg__ver">v{{ backend.version }}</span>
            <span class="build-dlg__meta">{{ backend.gitCommit }} · {{ formatBuildTime(backend.buildTime) }}</span>
          </dd>
          <dd v-else class="build-dlg__meta">Loading…</dd>
        </dl>

        <div class="dialog__actions">
          <button
            ref="closeBtn"
            type="button"
            class="btn-primary dialog__btn"
            @click="close"
          >Close</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useBuildInfo } from '@/composables/useBuildInfo'
import { useModal } from '@/composables/useModal'

const props = defineProps<{ open?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { frontend, backend } = useBuildInfo()
const closeBtn = ref<HTMLButtonElement | null>(null)

function close() { emit('close') }

function formatBuildTime(s: string): string {
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const parts = new Intl.DateTimeFormat(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
    timeZoneName: 'short',
  }).formatToParts(d)
  const get = (t: string) => parts.find(p => p.type === t)?.value ?? ''
  const tz = get('timeZoneName')
  return `${get('year')}-${get('month')}-${get('day')} ${get('hour')}:${get('minute')}${tz ? ` ${tz}` : ''}`
}

useModal(() => !!props.open, () => closeBtn.value)
</script>

<style scoped>
.build-dlg { text-align: left; }
.build-dlg__grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 14px;
  margin: 8px 0 4px;
  font-family: var(--font-mono);
  font-size: .82rem;
}
.build-dlg__grid dt {
  text-transform: uppercase;
  letter-spacing: .12em;
  font-weight: 700;
  font-family: var(--font-ui);
  font-size: .72rem;
  color: var(--muted);
  align-self: center;
}
.build-dlg__grid dd {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-wrap: anywhere;
}
.build-dlg__ver { color: var(--ink); font-weight: 700; }
.build-dlg__meta { color: var(--muted); font-size: .76rem; white-space: nowrap; }
</style>
