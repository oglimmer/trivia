<template>
  <div class="stack">
    <div class="photo-frame">
      <img v-if="modelValue" :src="modelValue" alt="" />
      <span v-else class="photo-frame__placeholder">no photo yet</span>
    </div>
    <div class="row">
      <button class="btn-warn flex-1" @click="pick('environment')">
        <span aria-hidden="true">📷</span> Camera
      </button>
      <button class="btn-ghost flex-1" @click="pick()">
        <span aria-hidden="true">🖼</span> Library
      </button>
      <button v-if="modelValue" class="btn-danger btn-icon" @click="emit('update:modelValue', '')" aria-label="Clear photo">✕</button>
    </div>
    <input ref="fileEl" type="file" accept="image/*" :capture="capture" @change="onFile" hidden />
    <div v-if="busy" class="helper">Processing image…</div>
    <div v-if="err" class="error">{{ err }}</div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  maxSize: { type: Number, default: 1024 },
})
const emit = defineEmits(['update:modelValue'])

const fileEl = ref(null)
const capture = ref(null)
const busy = ref(false)
const err = ref('')

function pick(cap) {
  capture.value = cap || null
  setTimeout(() => fileEl.value && fileEl.value.click(), 0)
}

async function onFile(e) {
  err.value = ''
  const f = e.target.files && e.target.files[0]
  if (!f) return
  busy.value = true
  try {
    const dataUrl = await fileToResizedDataURL(f, props.maxSize)
    emit('update:modelValue', dataUrl)
  } catch (ex) {
    err.value = ex.message
  } finally {
    busy.value = false
    e.target.value = ''
  }
}

function fileToResizedDataURL(file, maxSize) {
  return new Promise((resolve, reject) => {
    const fr = new FileReader()
    fr.onerror = () => reject(new Error('Could not read file'))
    fr.onload = () => {
      const img = new Image()
      img.onload = () => {
        const ratio = Math.min(1, maxSize / Math.max(img.width, img.height))
        const w = Math.round(img.width * ratio)
        const h = Math.round(img.height * ratio)
        const cvs = document.createElement('canvas')
        cvs.width = w
        cvs.height = h
        const ctx = cvs.getContext('2d')
        ctx.drawImage(img, 0, 0, w, h)
        resolve(cvs.toDataURL('image/jpeg', 0.82))
      }
      img.onerror = () => reject(new Error('Could not decode image'))
      img.src = fr.result
    }
    fr.readAsDataURL(file)
  })
}
</script>
