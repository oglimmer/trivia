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
        v-if="imageId && !allowRandom"
        class="btn-danger btn-icon"
        :disabled="busy"
        @click="clear"
        aria-label="Clear photo"
      >✕</button>
    </div>
    <div v-if="allowRandom && isRandom" class="helper helper--row">
      <span>🎲 Tap <strong>Random</strong> again to roll a new one</span>
      <button class="btn-link" @click="clear">Clear</button>
    </div>
    <div v-else-if="allowRandom && imageId" class="helper helper--row">
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
import { ref, type Ref } from 'vue'
import { imageUrl } from '@/services/images'

const props = withDefaults(defineProps<{
  imageId?: string
  maxSize?: number
  noFrame?: boolean
  allowRandom?: boolean
}>(), {
  imageId: '',
  maxSize: 1024,
  noFrame: false,
  allowRandom: false,
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

// --- Random avatar generator ---
const RANDOM_EMOJIS = [
  '🦊','🐼','🐙','🦄','🐸','🦖','🐵','🐯','🦁','🐨','🦝','🐺','🐱','🐶','🐰',
  '🐮','🐷','🐭','🐹','🐻','🐗','🦔','🐢','🐧','🦆','🦉','🦅','🦦','🦥','🐝',
  '🦋','🐞','🦑','🐠','🐡','🦀','🐬','🐳','🦈','🐲','🦕',
  '🍕','🌮','🍔','🍩','🍪','🧁','🍰','🍦','🍓','🍉','🥑','🍍','🍑','🍒','🍙','🍣',
  '🌈','⭐','🌟','✨','🎈','🎉','🎲','🎯','🎮','🎸','🎺','🪐','🌙','🔥','⚡',
  '🌊','🌸','🌺','🌻','🍄','🎨','🎭','🎃','🪄','🍭','🌵',
]

const RANDOM_BGS = [
  '#FFD8E5', '#FFF1B3', '#D9E7FF', '#CFF1EE', '#FBEFD0', '#FFB39C',
  '#FF4D8D', '#FFD23F', '#3A86FF', '#4ECDC4', '#FF6B6B',
]

const lastEmojiIdx = ref(-1)
const lastBgIdx = ref(-1)

function rollIdx(maxLen: number, lastRef: Ref<number>): number {
  let i: number
  do { i = Math.floor(Math.random() * maxLen) } while (maxLen > 1 && i === lastRef.value)
  lastRef.value = i
  return i
}

async function generateRandom() {
  if (busy.value) return
  err.value = ''
  const size = 512
  const cvs = document.createElement('canvas')
  cvs.width = size
  cvs.height = size
  const ctx = cvs.getContext('2d')
  if (!ctx) { err.value = 'Canvas unsupported'; return }

  const bg = RANDOM_BGS[rollIdx(RANDOM_BGS.length, lastBgIdx)]
  ctx.fillStyle = bg
  ctx.fillRect(0, 0, size, size)

  const pat = Math.random()
  if (pat < 0.33) {
    ctx.strokeStyle = 'rgba(26, 27, 38, 0.09)'
    ctx.lineWidth = size * 0.05
    ctx.beginPath()
    for (let i = -size; i < size * 2; i += size * 0.18) {
      ctx.moveTo(i, 0)
      ctx.lineTo(i + size, size)
    }
    ctx.stroke()
  } else if (pat < 0.66) {
    ctx.fillStyle = 'rgba(255, 255, 255, 0.45)'
    const step = size * 0.18
    for (let x = step / 2; x < size; x += step) {
      for (let y = step / 2; y < size; y += step) {
        ctx.beginPath()
        ctx.arc(x, y, size * 0.025, 0, Math.PI * 2)
        ctx.fill()
      }
    }
  }

  const emoji = RANDOM_EMOJIS[rollIdx(RANDOM_EMOJIS.length, lastEmojiIdx)]
  ctx.font = `${Math.round(size * 0.6)}px "Apple Color Emoji","Segoe UI Emoji","Noto Color Emoji",sans-serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(emoji, size / 2, size / 2 + size * 0.04)

  setBusy(true)
  try {
    const blob: Blob = await new Promise((resolve, reject) => {
      cvs.toBlob(
        (b) => b ? resolve(b) : reject(new Error('Could not generate avatar')),
        'image/png',
      )
    })
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
