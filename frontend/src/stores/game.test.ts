import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// Stub WS + API modules — the store imports them at module init time.
vi.mock('@/services/ws', () => ({
  onMessage: vi.fn(),
  wsConnectPlayer: vi.fn(),
  wsConnectBoard: vi.fn(),
}))
vi.mock('@/services/api', () => ({
  playerApi: { me: vi.fn() },
}))

import { useGameStore } from './game'
import { onMessage, wsConnectBoard, wsConnectPlayer } from '@/services/ws'
import { playerApi } from '@/services/api'
import type {
  Answer,
  AnswerAck,
  GameStateMsg,
  LeaderboardEntry,
  Question,
  User,
  Game,
  WSListener,
} from '@/types'

const user = (over: Partial<User> = {}): User => ({ id: 'u1', name: 'Alice', ...over })
const game = (over: Partial<Game> = {}): Game => ({
  code: 'ABCD',
  name: 'test',
  state: 'game',
  ...over,
})

describe('game store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('starts unauthenticated', () => {
    const s = useGameStore()
    expect(s.me).toBeNull()
    expect(s.game).toBeNull()
    expect(s.isPlayer).toBe(false)
    expect(s.isAdmin).toBe(false)
  })

  it('setMe persists the token and exposes the user', () => {
    const s = useGameStore()
    s.setMe('tok-123', user())
    expect(localStorage.getItem('playerToken')).toBe('tok-123')
    expect(s.me?.name).toBe('Alice')
    expect(s.isPlayer).toBe(true)
  })

  it('updateMe merges a partial user', () => {
    const s = useGameStore()
    s.setMe('t', user())
    s.updateMe({ name: 'Bob' })
    expect(s.me?.name).toBe('Bob')
    expect(s.me?.id).toBe('u1')
  })

  it('updateMe is a no-op without a current user', () => {
    const s = useGameStore()
    s.updateMe({ name: 'Bob' })
    expect(s.me).toBeNull()
  })

  it('isInGame / isFinished follow game.state', () => {
    const s = useGameStore()
    s.setGame(game({ state: 'setup' }))
    expect(s.isInGame).toBe(false)
    expect(s.isFinished).toBe(false)
    s.setGame(game({ state: 'game' }))
    expect(s.isInGame).toBe(true)
    s.setGame(game({ state: 'finished' }))
    expect(s.isFinished).toBe(true)
  })

  it('logout clears player state and token', () => {
    const s = useGameStore()
    s.setMe('t', user())
    s.setGame(game())
    s.logout()
    expect(localStorage.getItem('playerToken')).toBeNull()
    expect(s.me).toBeNull()
    expect(s.game).toBeNull()
    expect(s.wsStarted).toBe(false)
  })

  it('admin token round-trips through setAdmin / logoutAdmin', () => {
    const s = useGameStore()
    s.setAdmin('admin-tok')
    expect(localStorage.getItem('adminToken')).toBe('admin-tok')
    expect(s.isAdmin).toBe(true)
    s.logoutAdmin()
    expect(localStorage.getItem('adminToken')).toBeNull()
    expect(s.isAdmin).toBe(false)
  })

  it('loadMe populates from playerApi.me when a token exists', async () => {
    localStorage.setItem('playerToken', 'tok')
    vi.mocked(playerApi.me).mockResolvedValueOnce({ user: user(), game: game() })
    const s = useGameStore()
    await s.loadMe()
    expect(s.me?.id).toBe('u1')
    expect(s.game?.code).toBe('ABCD')
  })

  it('loadMe clears the token if the API rejects', async () => {
    localStorage.setItem('playerToken', 'tok')
    vi.mocked(playerApi.me).mockRejectedValueOnce(new Error('401'))
    const s = useGameStore()
    await s.loadMe()
    expect(localStorage.getItem('playerToken')).toBeNull()
    expect(s.me).toBeNull()
    expect(s.game).toBeNull()
  })

  it('ensureWS subscribes and connects exactly once', async () => {
    localStorage.setItem('playerToken', 'tok')
    const s = useGameStore()
    await s.ensureWS()
    await s.ensureWS()
    expect(onMessage).toHaveBeenCalledTimes(1)
    expect(wsConnectPlayer).toHaveBeenCalledExactlyOnceWith('tok')
  })

  it('ensureWS is a no-op without a token', async () => {
    const s = useGameStore()
    await s.ensureWS()
    expect(wsConnectPlayer).not.toHaveBeenCalled()
    expect(s.wsStarted).toBe(false)
  })

  describe('WS message handler', () => {
    // Capture the listener registered by ensureWS so we can drive the reducer.
    async function bootHandler() {
      localStorage.setItem('playerToken', 'tok')
      const s = useGameStore()
      await s.ensureWS()
      const handler = vi.mocked(onMessage).mock.calls[0][0] as WSListener
      return { s, handler }
    }

    it('_connected / _disconnected flip the connected flag', async () => {
      const { s, handler } = await bootHandler()
      handler({ type: '_connected' })
      expect(s.connected).toBe(true)
      handler({ type: '_disconnected' })
      expect(s.connected).toBe(false)
    })

    it('gameState updates code/name/state and copies through question + answers', async () => {
      const { s, handler } = await bootHandler()
      const q: Question = { id: 'q1', userId: 'u1', text: '2+2?', answerType: 'number', correct: 4 }
      const data: GameStateMsg = {
        code: 'ABCD',
        name: 'Quiz Night',
        state: 'game',
        questionState: 'active',
        currentQuestionId: 'q1',
        questionStartedAt: '2026-01-01T00:00:00Z',
        questionTimeoutSeconds: 20,
        scheduledAt: null,
        question: q,
        answers: [],
      }
      handler({ type: 'gameState', data })
      expect(s.game?.code).toBe('ABCD')
      expect(s.game?.state).toBe('game')
      expect(s.game?.questionTimeoutSeconds).toBe(20)
      expect(s.question?.id).toBe('q1')
      expect(s.answers).toEqual([])
    })

    it('gameState computes serverClockOffsetMs from serverNow', async () => {
      const { s, handler } = await bootHandler()
      vi.useFakeTimers()
      try {
        const now = new Date('2026-01-01T00:00:00Z').getTime()
        vi.setSystemTime(now)
        handler({
          type: 'gameState',
          data: {
            code: 'A', name: 'n', state: 'setup',
            serverNow: new Date(now + 1234).toISOString(),
          },
        })
        expect(s.serverClockOffsetMs).toBe(1234)
      } finally {
        vi.useRealTimers()
      }
    })

    it('gameState preserves prior leaderboard when none is included', async () => {
      const { s, handler } = await bootHandler()
      const lb: LeaderboardEntry[] = [{ userId: 'u1', userName: 'A', points: 10 }]
      handler({ type: 'gameState', data: { code: 'A', name: 'n', state: 'game', leaderboard: lb } })
      expect(s.leaderboard).toEqual(lb)
      // Second update without leaderboard should not wipe it.
      handler({ type: 'gameState', data: { code: 'A', name: 'n', state: 'game' } })
      expect(s.leaderboard).toEqual(lb)
    })

    it('users replaces the player roster', async () => {
      const { s, handler } = await bootHandler()
      const list: User[] = [{ id: 'u1', name: 'A' }, { id: 'u2', name: 'B' }]
      handler({ type: 'users', data: list })
      expect(s.users).toEqual(list)
      handler({ type: 'users', data: [] })
      expect(s.users).toEqual([])
    })

    it('answerAck stores the latest acknowledgement', async () => {
      const { s, handler } = await bootHandler()
      const ack: AnswerAck = { questionId: 'q1', responseMs: 800, isCorrect: true, points: 100 }
      handler({ type: 'answerAck', data: ack })
      expect(s.lastAnswerAck).toEqual(ack)
    })

    it('ignores unrelated message types without throwing', async () => {
      const { s, handler } = await bootHandler()
      handler({ type: 'pong' })
      handler({ type: 'presence', data: { online: ['u1'] } })
      // No public state changes — just make sure we didn't crash and state stayed default.
      expect(s.connected).toBeNull()
    })

    it('board connections track which teams have locked in', async () => {
      const s = useGameStore()
      s.ensureBoardWS('consensus')
      expect(vi.mocked(wsConnectBoard)).toHaveBeenCalledWith('consensus')
      const handler = vi.mocked(onMessage).mock.calls[0][0] as WSListener

      handler({ type: 'gameState', data: { code: 'consensus', name: 'n', state: 'game', currentQuestionId: 'q1', questionState: 'active' } })
      handler({ type: 'answeredSnapshot', data: { questionId: 'q1', userIds: ['u1', 'u2'] } })
      expect(s.answeredUserIds).toEqual(['u1', 'u2'])

      handler({ type: 'playerAnswered', data: { userId: 'u3', questionId: 'q1' } })
      expect(s.answeredUserIds).toEqual(['u1', 'u2', 'u3'])

      // Repeats must not double up — a reconnect replays the same event.
      handler({ type: 'playerAnswered', data: { userId: 'u3', questionId: 'q1' } })
      expect(s.answeredUserIds).toEqual(['u1', 'u2', 'u3'])

      // An event for a question that is no longer current is ignored.
      handler({ type: 'playerAnswered', data: { userId: 'u4', questionId: 'q-old' } })
      expect(s.answeredUserIds).toEqual(['u1', 'u2', 'u3'])

      // Moving to the next question clears the board.
      handler({ type: 'gameState', data: { code: 'consensus', name: 'n', state: 'game', currentQuestionId: 'q2', questionState: 'active' } })
      expect(s.answeredUserIds).toEqual([])
    })

    it('gameState carries the game mode through to the store', async () => {
      const { s, handler } = await bootHandler()
      handler({ type: 'gameState', data: { code: 'consensus', name: 'n', state: 'setup', mode: 'poll' } })
      expect(s.game?.mode).toBe('poll')
    })

    it('gameState answers default to empty array when missing', async () => {
      const { s, handler } = await bootHandler()
      const a: Answer[] = [{ id: 'a1', userId: 'u1', questionId: 'q1', answer: 1, isCorrect: true, points: 10, responseMs: 200 }]
      handler({ type: 'gameState', data: { code: 'A', name: 'n', state: 'game', answers: a } })
      expect(s.answers).toEqual(a)
      handler({ type: 'gameState', data: { code: 'A', name: 'n', state: 'game' } })
      expect(s.answers).toEqual([])
    })
  })
})
