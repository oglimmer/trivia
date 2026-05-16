import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useQuestionCountdown } from './useQuestionCountdown'
import type { Game } from '@/types'

// Mounts the composable inside a throwaway component so onUnmounted/watch run.
function harness(game: Ref<Game | null>, offset: Ref<number>) {
  const Host = defineComponent({
    setup() {
      return useQuestionCountdown(game, { serverClockOffsetMs: offset, intervalMs: 100 })
    },
    template: '<div />',
  })
  return mount(Host)
}

const T0 = new Date('2026-01-01T00:00:00Z').getTime()

function activeGame(overrides: Partial<Game> = {}): Game {
  return {
    code: 'ABC',
    name: 'test',
    state: 'game',
    questionState: 'active',
    currentQuestionId: 'q1',
    questionStartedAt: new Date(T0).toISOString(),
    questionTimeoutSeconds: 30,
    ...overrides,
  }
}

describe('useQuestionCountdown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(T0)
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('reports full remaining time at start', () => {
    const game = ref<Game | null>(activeGame())
    const offset = ref(0)
    const w = harness(game, offset)
    expect(w.vm.remaining).toBe(30)
    expect(w.vm.ringPct).toBe(100)
  })

  it('decrements as time advances', async () => {
    const game = ref<Game | null>(activeGame())
    const offset = ref(0)
    const w = harness(game, offset)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(w.vm.remaining).toBe(20)
    expect(w.vm.ringPct).toBe(67)
  })

  it('clamps to zero past the deadline', async () => {
    const game = ref<Game | null>(activeGame({ questionTimeoutSeconds: 5 }))
    const offset = ref(0)
    const w = harness(game, offset)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(w.vm.remaining).toBe(0)
    expect(w.vm.ringPct).toBe(0)
  })

  it('applies server clock offset', async () => {
    const game = ref<Game | null>(activeGame())
    // Client clock is 5s behind server -> remaining should reflect server-side time.
    const offset = ref(5_000)
    const w = harness(game, offset)
    // Initial tick reads Date.now() + offset.
    expect(w.vm.remaining).toBe(25)
  })

  it('stops ticking once questionState leaves active', async () => {
    const game = ref<Game | null>(activeGame())
    const offset = ref(0)
    const w = harness(game, offset)
    game.value = { ...game.value!, questionState: 'revealed' }
    await vi.advanceTimersByTimeAsync(100)
    expect(w.vm.remaining).toBe(0)
    expect(w.vm.ringPct).toBe(0)
  })

  it('resets when a new question starts', async () => {
    const game = ref<Game | null>(activeGame())
    const offset = ref(0)
    const w = harness(game, offset)
    await vi.advanceTimersByTimeAsync(20_000)
    expect(w.vm.remaining).toBe(10)
    // New question begins now.
    game.value = activeGame({ questionStartedAt: new Date(Date.now()).toISOString() })
    await vi.advanceTimersByTimeAsync(0)
    expect(w.vm.remaining).toBe(30)
  })
})
