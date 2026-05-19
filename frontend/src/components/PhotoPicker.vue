<template>
  <div class="stack">
    <div v-if="!noFrame" class="photo-frame">
      <img
        v-if="imageId"
        :src="imageUrl(imageId, 'medium')"
        alt=""
        loading="lazy"
        decoding="async"
      />
      <span v-else class="photo-frame__placeholder">no photo yet</span>
    </div>
    <div v-if="allowRotate && imageId" class="row">
      <button class="btn-ghost flex-1" :disabled="busy" @click="rotateImage(-90)" aria-label="Rotate left">
        ↺ Rotate left
      </button>
      <button class="btn-ghost flex-1" :disabled="busy" @click="rotateImage(90)" aria-label="Rotate right">
        ↻ Rotate right
      </button>
    </div>
    <div :class="allowRandom ? 'picker-actions' : 'row'">
      <button class="btn-warn flex-1" :disabled="busy" @click="pick('environment')">
        <span aria-hidden="true">📷</span> Camera
      </button>
      <button class="btn-ghost flex-1" :disabled="busy" @click="pick()">
        <span aria-hidden="true">🖼</span> Library
      </button>
      <button
        v-if="allowRandom"
        class="btn-accent"
        :disabled="busy"
        @click="generateRandom"
        :title="isRandom ? 'Roll a different one' : 'Pick a random icon'"
      >
        <span aria-hidden="true">🎲</span> Random
      </button>
      <button
        v-if="imageId && !allowRandom && !noClear"
        class="btn-danger btn-icon"
        :disabled="busy"
        @click="clear"
        aria-label="Clear photo"
      >✕</button>
    </div>
    <div v-if="allowRandom && isRandom" class="helper helper--row">
      <span>🎲 Tap <strong>Random</strong> again to roll a new one</span>
      <button v-if="!noClear" class="btn-link" @click="clear">Clear</button>
    </div>
    <div v-else-if="allowRandom && imageId && !noClear" class="helper helper--row">
      <span></span>
      <button class="btn-link" @click="clear">Clear photo</button>
    </div>
    <input ref="fileEl" type="file" accept="image/*" :capture="capture || undefined" @change="onFile" hidden />
    <div v-if="busy" class="helper picker-busy">
      <span class="picker-busy__dot" aria-hidden="true"></span>
      Uploading image…
    </div>
    <div v-if="err" class="error">{{ err }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { imageUrl } from '@/services/images'
import { generateRandomAvatarBlob } from '@/utils/randomAvatar'

const props = withDefaults(defineProps<{
  imageId?: string
  maxSize?: number
  noFrame?: boolean
  allowRandom?: boolean
  allowRotate?: boolean
  noClear?: boolean
}>(), {
  imageId: '',
  maxSize: 1024,
  noFrame: false,
  allowRandom: false,
  allowRotate: false,
  noClear: false,
})
const emit = defineEmits<{
  (e: 'update:imageId', v: string): void
  (e: 'busy', v: boolean): void
}>()

const fileEl = ref<HTMLInputElement | null>(null)
const capture = ref<'user' | 'environment' | null>(null)
const busy = ref(false)
const err = ref('')
const isRandom = ref(false)

function setBusy(v: boolean) {
  busy.value = v
  emit('busy', v)
}

function pick(cap?: 'user' | 'environment') {
  capture.value = cap || null
  setTimeout(() => fileEl.value && fileEl.value.click(), 0)
}

function clear() {
  isRandom.value = false
  err.value = ''
  emit('update:imageId', '')
}

async function onFile(e: Event) {
  err.value = ''
  const target = e.target as HTMLInputElement
  const f = target.files && target.files[0]
  if (!f) return
  setBusy(true)
  try {
    const blob = await fileToResizedJpegBlob(f, props.maxSize)
    const id = await uploadImage(blob)
    isRandom.value = false
    emit('update:imageId', id)
  } catch (ex) {
    err.value = (ex as Error).message || 'Upload failed'
  } finally {
    setBusy(false)
    target.value = ''
  }
}

function fileToResizedJpegBlob(file: File, maxSize: number): Promise<Blob> {
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
        if (!ctx) { reject(new Error('Canvas unsupported')); return }
        ctx.drawImage(img, 0, 0, w, h)
        cvs.toBlob(
          (b) => b ? resolve(b) : reject(new Error('Could not encode image')),
          'image/jpeg',
          0.82,
        )
      }
      img.onerror = () => reject(new Error('Could not decode image'))
      img.src = fr.result as string
    }
    fr.readAsDataURL(file)
  })
}

async function uploadImage(blob: Blob): Promise<string> {
  const form = new FormData()
  form.append('file', blob, 'photo.jpg')
  const r = await fetch('/api/images', { method: 'POST', body: form })
  if (!r.ok) {
    if (r.status === 413) throw new Error('Image is too large')
    let msg = ''
    try { msg = (await r.json()).error || '' } catch {
      try { msg = await r.text() } catch {}
    }
    throw new Error(msg || 'Upload failed')
  }
  const j = await r.json() as { id: string }
  return j.id
}

function loadImageEl(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('Could not load image'))
    img.src = url
  })
}

function canvasToBlob(cvs: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    cvs.toBlob(
      (b) => b ? resolve(b) : reject(new Error('Could not encode image')),
      'image/jpeg',
      0.82,
    )
  })
}

async function rotateImage(deg: 90 | -90) {
  if (!props.imageId || busy.value) return
  err.value = ''
  setBusy(true)
  try {
    const img = await loadImageEl(imageUrl(props.imageId, 'medium'))
    const cvs = document.createElement('canvas')
    cvs.width = img.height
    cvs.height = img.width
    const ctx = cvs.getContext('2d')
    if (!ctx) throw new Error('Canvas unsupported')
    ctx.translate(cvs.width / 2, cvs.height / 2)
    ctx.rotate((deg * Math.PI) / 180)
    ctx.drawImage(img, -img.width / 2, -img.height / 2)
    const blob = await canvasToBlob(cvs)
    const id = await uploadImage(blob)
    emit('update:imageId', id)
  } catch (ex) {
    err.value = (ex as Error).message || 'Rotate failed'
  } finally {
    setBusy(false)
  }
}

async function generateRandom() {
  if (busy.value) return
  err.value = ''
  setBusy(true)
  try {
    const blob = await generateRandomAvatarBlob()
    const id = await uploadImage(blob)
    isRandom.value = true
    emit('update:imageId', id)
  } catch (ex) {
    err.value = (ex as Error).message || 'Could not generate avatar'
  } finally {
    setBusy(false)
  }
}
</script>

<style scoped>
.picker-busy {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.picker-busy__dot {
  display: inline-block;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid var(--ink);
  border-right-color: transparent;
  animation: picker-spin .9s linear infinite;
}
@keyframes picker-spin { to { transform: rotate(360deg); } }
</style>
