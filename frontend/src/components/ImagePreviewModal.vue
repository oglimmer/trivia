<template>
  <transition name="dialog">
    <div
      v-if="src"
      class="img-preview-backdrop"
      role="dialog"
      aria-modal="true"
      aria-label="Photo preview"
      @mousedown.self="emit('close')"
      @keydown.esc.prevent="emit('close')"
      tabindex="-1"
      ref="backdrop"
    >
      <button
        type="button"
        class="img-preview-close"
        aria-label="Close preview"
        @click="emit('close')"
      >×</button>
      <img class="img-preview-img" :src="src" alt="" />
    </div>
  </transition>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useModalRef } from '@/composables/useModal'

const props = defineProps<{ src: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const backdrop = ref<HTMLElement | null>(null)
const isOpen = computed(() => !!props.src)
useModalRef(isOpen, () => backdrop.value)
</script>

<style scoped>
.img-preview-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(26, 27, 38, .85);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 1100;
  cursor: zoom-out;
}
.img-preview-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-lg);
  background: var(--paper);
  box-shadow: var(--shadow-3);
  cursor: default;
}
.img-preview-close {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: var(--bw) solid var(--ink);
  background: var(--paper);
  color: var(--ink);
  font-size: 1.5rem;
  font-weight: 800;
  line-height: 1;
  cursor: pointer;
  box-shadow: var(--shadow-1);
}
.img-preview-close:hover { background: var(--coral); color: var(--paper); }
</style>
