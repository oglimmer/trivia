import { defineStore } from 'pinia'
import { onMessage, wsConnectPlayer } from '../services/ws'
import { api } from '../services/api'
import type {
  Answer,
  AnswerAck,
  Game,
  LeaderboardEntry,
  Question,
  User,
  WSMessage,
} from '../types'

interface GameStoreState {
  connected: boolean | null
  me: User | null
  game: Game | null
  question: Question | null
  leaderboard: LeaderboardEntry[]
  users: User[]
  answers: Answer[]
  lastAnswerAck: AnswerAck | null
  wsStarted: boolean
  isAdmin: boolean
  // serverClockOffsetMs = serverTime - clientTime, refreshed on each gameState.
  // Used by the question countdown so a skewed local clock doesn't bias the timer.
  serverClockOffsetMs: number
}

export const useGameStore = defineStore('game', {
  state: (): GameStoreState => ({
    connected: null,
    me: null,
    game: null,
    question: null,
    leaderboard: [],
    users: [],
    answers: [],
    lastAnswerAck: null,
    wsStarted: false,
    isAdmin: !!localStorage.getItem('adminToken'),
    serverClockOffsetMs: 0,
  }),
  getters: {
    isPlayer: (s): boolean => !!s.me,
    isInGame: (s): boolean => !!(s.game && s.game.state === 'game'),
    isFinished: (s): boolean => !!(s.game && s.game.state === 'finished'),
  },
  actions: {
    async ensureWS(): Promise<void> {
      if (this.wsStarted) return
      const tok = localStorage.getItem('playerToken')
      if (!tok) return
      onMessage((m) => this._handle(m))
      wsConnectPlayer(tok)
      this.wsStarted = true
    },
    async loadMe(): Promise<void> {
      const tok = localStorage.getItem('playerToken')
      if (!tok) { this.me = null; this.game = null; return }
      try {
        const r = await api.me()
        this.me = r.user
        this.game = r.game
      } catch {
        localStorage.removeItem('playerToken')
        this.me = null
        this.game = null
      }
    },
    setMe(token: string, user: User): void {
      localStorage.setItem('playerToken', token)
      this.me = user
    },
    logout(): void {
      localStorage.removeItem('playerToken')
      this.me = null
      this.game = null
      this.wsStarted = false
    },
    setAdmin(token: string): void {
      localStorage.setItem('adminToken', token)
      this.isAdmin = true
    },
    logoutAdmin(): void {
      localStorage.removeItem('adminToken')
      this.isAdmin = false
    },
    _handle(m: WSMessage): void {
      switch (m.type) {
        case '_connected': this.connected = true; break
        case '_disconnected': this.connected = false; break
        case 'gameState': {
          const d = m.data
          if (d.serverNow) {
            this.serverClockOffsetMs = new Date(d.serverNow).getTime() - Date.now()
          }
          this.game = {
            ...(this.game || {} as Game),
            code: d.code,
            name: d.name,
            state: d.state,
            questionState: d.questionState,
            currentQuestionId: d.currentQuestionId,
            questionStartedAt: d.questionStartedAt,
            questionTimeoutSeconds: d.questionTimeoutSeconds,
          }
          this.question = d.question || null
          if (d.leaderboard) this.leaderboard = d.leaderboard
          this.answers = d.answers || []
          break
        }
        case 'users':
          this.users = m.data ?? []
          break
        case 'answerAck':
          this.lastAnswerAck = m.data
          break
      }
    },
  },
})
