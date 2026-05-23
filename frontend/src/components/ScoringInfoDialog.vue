<template>
  <transition name="dialog">
    <div
      v-if="open"
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      aria-labelledby="scoring-dlg-title"
      @mousedown.self="close"
      @keydown.esc.prevent="close"
    >
      <div class="modal stack scoring-dlg">
        <div class="dialog__icon" aria-hidden="true">🎯</div>
        <h2 id="scoring-dlg-title" class="dialog__title">How scoring works</h2>

        <p class="scoring-dlg__lead">
          Every correct answer earns <strong>base points</strong>, plus a
          <strong>speed bonus</strong> for quick fingers.
        </p>

        <dl class="scoring-dlg__grid" aria-label="Base points by answer type">
          <dt>Yes / No · 2 options</dt><dd>100</dd>
          <dt>3 options</dt><dd>200</dd>
          <dt>4 options</dt><dd>300</dd>
          <dt>Number question</dt><dd>300</dd>
        </dl>

        <p class="scoring-dlg__note">
          <span aria-hidden="true">⚡</span>
          Speed bonus: up to <strong>half the base</strong> if you answer
          instantly, fading to <strong>0 at 30 s</strong>.
        </p>

        <p class="scoring-dlg__note">
          <span aria-hidden="true">🎲</span>
          Number guesses: only the <strong>3 closest</strong> score — closer
          guesses earn more. Spot-on guesses get the speed bonus too.
        </p>

        <div class="scoring-dlg__example">
          <span class="scoring-dlg__example-label">Example</span>
          4-option question, correct, answered in 6 s
          → 300 + 120 = <strong>420</strong>
        </div>

        <div class="dialog__actions">
          <button
            ref="closeBtn"
            type="button"
            class="btn-primary dialog__btn"
            @click="close"
          >Got it</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useModal } from '@/composables/useModal'

const props = defineProps<{ open?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const closeBtn = ref<HTMLButtonElement | null>(null)

function close() { emit('close') }

useModal(() => !!props.open, () => closeBtn.value)
</script>

<style scoped>
.scoring-dlg { text-align: left; }
.scoring-dlg .dialog__icon,
.scoring-dlg .dialog__title { align-self: center; }
.scoring-dlg__lead {
  margin: 4px 0 12px;
  color: var(--ink);
  line-height: 1.45;
  text-align: center;
}
.scoring-dlg__grid {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 6px 14px;
  margin: 2px 0 14px;
  padding: 12px 14px;
  background: var(--cream);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
}
.scoring-dlg__grid dt {
  margin: 0;
  font-weight: 600;
  color: var(--ink);
}
.scoring-dlg__grid dd {
  margin: 0;
  font-family: var(--font-ui);
  font-weight: 900;
  font-size: 1.05rem;
  color: var(--ink);
  text-align: right;
}
.scoring-dlg__note {
  margin: 0 0 10px;
  padding-left: 4px;
  color: var(--ink);
  line-height: 1.45;
}
.scoring-dlg__note span[aria-hidden] {
  display: inline-block;
  margin-right: 6px;
}
.scoring-dlg__example {
  margin-top: 4px;
  padding: 12px 14px;
  background: var(--yellow);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
  font-weight: 600;
  line-height: 1.4;
}
.scoring-dlg__example-label {
  display: block;
  font-family: var(--font-ui);
  font-weight: 900;
  text-transform: uppercase;
  letter-spacing: .12em;
  font-size: .72rem;
  color: var(--ink-soft);
  margin-bottom: 4px;
}
</style>
