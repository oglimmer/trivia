import { onUnmounted, ref, watch, type Ref } from 'vue'
import type { Game } from '@/types'

interface Options {
  // Server-anchored clock offset (ms). When the server emits `serverNow`,
  // (serverNow - clientNow) is fed in so the countdown isn't biased by a
  // skewed local clock.
  serverClockOffsetMs: Ref<number> | (() => number)
  // Tick frequency in ms. Default 200ms is smooth enough for a ring + integer
  // seconds; 250ms is fine for an integer-only display.
  intervalMs?: number
}

interface Countdown {
  remaining: Ref<number>
  ringPct: Ref<number>
}

// Drives a per-question countdown timer off of `game.questionStartedAt` and
// `game.questionTimeoutSeconds`. Stops when questionState !== 'active'.
// Reset on every change to questionState or questionStartedAt.
export function useQuestionCountdown(
  game: Ref<Game | null>,
  opts: Options,
): Countdown {
  const remaining = ref(0)
  const ringPct = ref(100)
  const intervalMs = opts.intervalMs ?? 200
  let handle: ReturnType<typeof setInterval> | null = null

  const offset = () => typeof opts.serverClockOffsetMs === 'function'
    ? opts.serverClockOffsetMs()
    : opts.serverClockOffsetMs.value

  function stop() {
    if (handle) { clearInterval(handle); handle = null }
  }

  function start() {
    stop()
    const g = game.value
    if (!g || g.questionState !== 'active') {
      remaining.value = 0
      ringPct.value = 0
      return
    }
    const startedAt = g.questionStartedAt ? new Date(g.questionStartedAt).getTime() : Date.now()
    const total = g.questionTimeoutSeconds || 30
    const tick = () => {
      const elapsed = (Date.now() + offset() - startedAt) / 1000
      const left = Math.max(0, total - elapsed)
      remaining.value = Math.max(0, Math.ceil(left))
      ringPct.value = Math.round((left / total) * 100)
    }
    tick()
    handle = setInterval(tick, intervalMs)
  }

  watch(
    () => [game.value?.questionState, game.value?.questionStartedAt] as const,
    start,
    { immediate: true },
  )

  onUnmounted(stop)

  return { remaining, ringPct }
}
