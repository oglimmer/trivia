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

        <label for="profile-dlg-email">Email (optional)</label>
        <input
          id="profile-dlg-email"
          v-model="email"
          type="email"
          maxlength="120"
          placeholder="you@example.com"
          autocomplete="email"
          inputmode="email"
        />
        <p class="muted profile-dlg__email-hint">
          We email a one-click link so you can rejoin from any device.
        </p>

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
import { ref, computed, watch } from 'vue'
import PhotoPicker from './PhotoPicker.vue'
import { playerApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { useModal } from '@/composables/useModal'
import { errMsg } from '@/composables/errMsg'

const props = defineProps<{ open?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const store = useGameStore()

const name = ref('')
const photo = ref('')
const email = ref('')
const err = ref('')
const saving = ref(false)
const nameEl = ref<HTMLInputElement | null>(null)

const canSave = computed(() => {
  const trimmed = name.value.trim()
  if (!trimmed) return false
  const curName = store.me?.name || ''
  const curPhoto = store.me?.photoB64 || ''
  const curEmail = store.me?.email || ''
  return trimmed !== curName
    || photo.value !== curPhoto
    || email.value.trim() !== curEmail
})

// Reset fields when opening; useModal handles focus + body scroll.
watch(() => props.open, (v) => {
  if (v) {
    name.value = store.me?.name || ''
    photo.value = store.me?.photoB64 || ''
    email.value = store.me?.email || ''
    err.value = ''
    saving.value = false
  }
})

useModal(() => !!props.open, () => nameEl.value)

function close() {
  if (saving.value) return
  emit('close')
}

async function save() {
  err.value = ''
  saving.value = true
  try {
    const newName = name.value.trim()
    const newEmail = email.value.trim()
    await playerApi.updateMe({ name: newName, photoB64: photo.value, email: newEmail })
    store.updateMe({ name: newName, photoB64: photo.value, email: newEmail })
    emit('close')
  } catch (e) {
    err.value = errMsg(e, 'Could not save')
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
.profile-dlg__email-hint {
  margin: -4px 0 0;
  font-size: .85rem;
}
</style>
