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

// Keep state across calls so consecutive rolls don't repeat the same emoji/bg.
let lastEmojiIdx = -1
let lastBgIdx = -1

function rollIdx(maxLen: number, last: number): number {
  let i: number
  do { i = Math.floor(Math.random() * maxLen) } while (maxLen > 1 && i === last)
  return i
}

// Renders a random emoji-on-coloured-background avatar to a PNG Blob.
// Throws if the canvas API is unavailable.
export async function generateRandomAvatarBlob(size = 512): Promise<Blob> {
  const cvs = document.createElement('canvas')
  cvs.width = size
  cvs.height = size
  const ctx = cvs.getContext('2d')
  if (!ctx) throw new Error('Canvas unsupported')

  const bgIdx = rollIdx(RANDOM_BGS.length, lastBgIdx)
  lastBgIdx = bgIdx
  ctx.fillStyle = RANDOM_BGS[bgIdx]
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

  const emojiIdx = rollIdx(RANDOM_EMOJIS.length, lastEmojiIdx)
  lastEmojiIdx = emojiIdx
  ctx.font = `${Math.round(size * 0.6)}px "Apple Color Emoji","Segoe UI Emoji","Noto Color Emoji",sans-serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(RANDOM_EMOJIS[emojiIdx], size / 2, size / 2 + size * 0.04)

  return new Promise<Blob>((resolve, reject) => {
    cvs.toBlob(
      (b) => b ? resolve(b) : reject(new Error('Could not generate avatar')),
      'image/png',
    )
  })
}
