const wrongSoundUrls = Object.values(
  import.meta.glob<string>('../assets/sounds/wrong/*.mp3', {
    eager: true,
    query: '?url',
    import: 'default',
  }),
)

// Plays a random "wrong" SFX, using a shuffled queue so each clip plays once
// before any repeats, and never the same clip twice in a row across reshuffles.
export function useWrongSoundEffect() {
  let queue: string[] = []
  let lastPlayed: string | null = null

  function refill() {
    const next = [...wrongSoundUrls]
    for (let i = next.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1))
      ;[next[i], next[j]] = [next[j], next[i]]
    }
    if (next.length > 1 && next[0] === lastPlayed) {
      ;[next[0], next[1]] = [next[1], next[0]]
    }
    queue = next
  }

  function play() {
    if (wrongSoundUrls.length === 0) return
    if (queue.length === 0) refill()
    const url = queue.shift()!
    lastPlayed = url
    const audio = new Audio(url)
    audio.play().catch(() => {})
  }

  return { play }
}
