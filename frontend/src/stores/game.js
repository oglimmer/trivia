import { defineStore } from 'pinia'
import { onMessage, wsConnectPlayer } from '../services/ws.js'
import { api } from '../services/api.js'

export const useGameStore = defineStore('game', {
  state: () => ({
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
  }),
  getters: {
    isPlayer: (s) => !!s.me,
    isInGame: (s) => s.game && s.game.state === 'game',
    isFinished: (s) => s.game && s.game.state === 'finished',
  },
  actions: {
    async ensureWS() {
      if (this.wsStarted) return
      const tok = localStorage.getItem('playerToken')
      if (!tok) return
      onMessage((m) => this._handle(m))
      wsConnectPlayer(tok)
      this.wsStarted = true
    },
    async loadMe() {
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
    setMe(token, user) {
      localStorage.setItem('playerToken', token)
      this.me = user
    },
    logout() {
      localStorage.removeItem('playerToken')
      this.me = null
      this.game = null
      this.wsStarted = false
    },
    setAdmin(token) {
      localStorage.setItem('adminToken', token)
      this.isAdmin = true
    },
    logoutAdmin() {
      localStorage.removeItem('adminToken')
      this.isAdmin = false
    },
    _handle(m) {
      switch (m.type) {
        case '_connected': this.connected = true; break
        case '_disconnected': this.connected = false; break
        case 'gameState': {
          const d = m.data
          this.game = {
            ...(this.game || {}),
            code: d.code,
            name: d.name,
            state: d.state,
            questionState: d.questionState,
            currentQuestionId: d.currentQuestionId,
            questionStartedAt: d.questionStartedAt,
          }
          this.question = d.question || null
          if (d.leaderboard) this.leaderboard = d.leaderboard
          this.answers = d.answers || []
          break
        }
        case 'users':
          this.users = m.data
          break
        case 'answerAck':
          this.lastAnswerAck = m.data
          break
      }
    },
  },
})
