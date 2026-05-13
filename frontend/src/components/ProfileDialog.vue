<template>
  <transition name="dialog">
    <div
      v-if="open"
      class="modal-backdrop"
      role="dialog"
      aria-modal="true"
      aria-labelledby="profile-dlg-title"
      @mousedown.self="close"
      @keydown.esc.prevent="close"
    >
      <div class="modal stack profile-dlg">
        <h2 id="profile-dlg-title" class="dialog__title">Edit your card</h2>

        <div class="profile-dlg__preview">
          <img v-if="photo" class="avatar avatar-lg" :src="photo" alt="" />
          <span v-else class="avatar avatar-lg profile-dlg__placeholder" aria-hidden="true">🙂</span>
        </div>

        <label for="profile-dlg-name">Display name</label>
        <input
          id="profile-dlg-name"
          ref="nameEl"
          v-model="name"
          maxlength="40"
          placeholder="Your name"
          autocomplete="given-name"
        />

        <label>Selfie</label>
        <PhotoPicker v-model="photo" no-frame allow-random />

        <div v-if="err" class="error">{{ err }}</div>

        <div class="dialog__actions">
          <button
            type="button"
            class="btn-ghost dialog__btn"
            :disabled="saving"
            @click="close"
          >Cancel</button>
          <button
            type="button"
            class="btn-primary dialog__btn"
            :disabled="!canSave || saving"
            @click="save"
          >{{ saving ? 'Saving…' : 'Save' }}</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'
import PhotoPicker from './PhotoPicker.vue'
import { api } from '../services/api'
import { useGameStore } from '../stores/game'
import type { User } from '../types'

const props = defineProps<{ open?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const store = useGameStore()

const name = ref('')
const photo = ref('')
const err = ref('')
const saving = ref(false)
const nameEl = ref<HTMLInputElement | null>(null)

let prevFocus: Element | null = null
let prevOverflow = ''

const canSave = computed(() => {
  const trimmed = name.value.trim()
  if (!trimmed) return false
  const curName = store.me?.name || ''
  const curPhoto = store.me?.photoB64 || ''
  return trimmed !== curName || photo.value !== curPhoto
})

watch(() => props.open, async (v) => {
  if (v) {
    name.value = store.me?.name || ''
    photo.value = store.me?.photoB64 || ''
    err.value = ''
    saving.value = false
    prevFocus = document.activeElement
    prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
    nameEl.value?.focus()
  } else {
    document.body.style.overflow = prevOverflow
    if (prevFocus && 'focus' in prevFocus && typeof (prevFocus as HTMLElement).focus === 'function') (prevFocus as HTMLElement).focus()
    prevFocus = null
  }
})

onUnmounted(() => {
  if (props.open) document.body.style.overflow = prevOverflow
})

function close() {
  if (saving.value) return
  emit('close')
}

async function save() {
  err.value = ''
  saving.value = true
  try {
    const newName = name.value.trim()
    await api.updateMe({ name: newName, photoB64: photo.value })
    store.me = { ...(store.me || {} as User), name: newName, photoB64: photo.value }
    emit('close')
  } catch (e) {
    err.value = (e as Error).message || 'Could not save'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.profile-dlg { text-align: left; }
.profile-dlg__preview {
  display: flex;
  justify-content: center;
  margin: 2px 0 6px;
}
.profile-dlg__placeholder {
  display: inline-grid;
  place-items: center;
  font-size: 2.4rem;
  background: var(--cream-2);
}
</style>
