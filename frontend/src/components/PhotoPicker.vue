<template>
  <div class="stack">
    <div v-if="!noFrame" class="photo-frame">
      <img v-if="modelValue" :src="modelValue" alt="" />
      <span v-else class="photo-frame__placeholder">no photo yet</span>
    </div>
    <div :class="allowRandom ? 'picker-actions' : 'row'">
      <button class="btn-warn flex-1" @click="pick('environment')">
        <span aria-hidden="true">📷</span> Camera
      </button>
      <button class="btn-ghost flex-1" @click="pick()">
        <span aria-hidden="true">🖼</span> Library
      </button>
      <button
        v-if="allowRandom"
        class="btn-accent"
        @click="generateRandom"
        :title="isRandom ? 'Roll a different one' : 'Pick a random icon'"
      >
        <span aria-hidden="true">🎲</span> Random
      </button>
      <button
        v-if="modelValue && !allowRandom"
        class="btn-danger btn-icon"
        @click="clear"
        aria-label="Clear photo"
      >✕</button>
    </div>
    <div v-if="allowRandom && isRandom" class="helper helper--row">
      <span>🎲 Tap <strong>Random</strong> again to roll a new one</span>
      <button class="btn-link" @click="clear">Clear</button>
    </div>
    <div v-else-if="allowRandom && modelValue" class="helper helper--row">
      <span></span>
      <button class="btn-link" @click="clear">Clear photo</button>
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
  noFrame: { type: Boolean, default: false },
  allowRandom: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const fileEl = ref(null)
const capture = ref(null)
const busy = ref(false)
const err = ref('')
const isRandom = ref(false)

function pick(cap) {
  capture.value = cap || null
  setTimeout(() => fileEl.value && fileEl.value.click(), 0)
}

function clear() {
  isRandom.value = false
  emit('update:modelValue', '')
}

async function onFile(e) {
  err.value = ''
  const f = e.target.files && e.target.files[0]
  if (!f) return
  busy.value = true
  try {
    const dataUrl = await fileToResizedDataURL(f, props.maxSize)
    isRandom.value = false
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

function rollIdx(maxLen, lastRef) {
  let i
  do { i = Math.floor(Math.random() * maxLen) } while (maxLen > 1 && i === lastRef.value)
  lastRef.value = i
  return i
}

function generateRandom() {
  err.value = ''
  const size = 512
  const cvs = document.createElement('canvas')
  cvs.width = size
  cvs.height = size
  const ctx = cvs.getContext('2d')

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

  try {
    isRandom.value = true
    emit('update:modelValue', cvs.toDataURL('image/png'))
  } catch (ex) {
    err.value = 'Could not generate avatar'
  }
}
</script>
