<template>
  <transition name="dialog">
    <div
      v-if="d"
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      :aria-describedby="msgId"
      @mousedown.self="cancel"
      @keydown.esc.prevent="cancel"
      @keydown.enter.prevent="confirm"
    >
      <div class="modal card stack dialog" :class="`dialog--${d.tone}`">
        <div class="dialog__icon" aria-hidden="true">{{ d.icon }}</div>
        <h2 :id="titleId" class="dialog__title">{{ d.title }}</h2>
        <p v-if="d.message" :id="msgId" class="dialog__msg">{{ d.message }}</p>
        <div class="dialog__actions">
          <button
            ref="cancelBtn"
            type="button"
            class="btn-ghost dialog__btn"
            @click="cancel"
          >{{ d.cancelLabel }}</button>
          <button
            ref="confirmBtn"
            type="button"
            :class="['dialog__btn', d.tone === 'danger' ? 'btn-danger' : 'btn-primary']"
            @click="confirm"
          >{{ d.confirmLabel }}</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { dialogState, resolveDialog } from '@/services/dialog'
import { useModal } from '@/composables/useModal'

const d = computed(() => dialogState.current)
const cancelBtn = ref<HTMLButtonElement | null>(null)
const confirmBtn = ref<HTMLButtonElement | null>(null)
const titleId = 'dlg-title'
const msgId = 'dlg-msg'

function confirm() { resolveDialog(true) }
function cancel() { resolveDialog(false) }

useModal(
  () => !!d.value,
  () => (d.value?.tone === 'danger' ? cancelBtn.value : confirmBtn.value),
)
</script>
