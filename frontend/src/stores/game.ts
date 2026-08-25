import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { onMessage, wsConnectBoard, wsConnectPlayer } from '@/services/ws'
import { playerApi } from '@/services/api'
import type {
  Answer,
  AnswerAck,
  Game,
  LeaderboardEntry,
  Question,
  User,
  WSMessage,
} from '@/types'

export const useGameStore = defineStore('game', () => {
  const connected = ref<boolean | null>(null)
  const me = ref<User | null>(null)
  const game = ref<Game | null>(null)
  const question = ref<Question | null>(null)
  const leaderboard = ref<LeaderboardEntry[]>([])
  const users = ref<User[]>([])
  const answers = ref<Answer[]>([])
  const lastAnswerAck = ref<AnswerAck | null>(null)
  // Teams that have locked in an answer to the current question. Fed by the
  // board's playerAnswered/answeredSnapshot messages so the TV can light names
  // up live; stays empty for player connections, which never receive them.
  const answeredUserIds = ref<string[]>([])
  const wsStarted = ref(false)
  const isAdmin = ref(!!localStorage.getItem('adminToken'))
  // serverClockOffsetMs = serverTime - clientTime, refreshed on each gameState.
  // Used by the question countdown so a skewed local clock doesn't bias the timer.
  const serverClockOffsetMs = ref(0)

  const isPlayer = computed(() => !!me.value)
  const isInGame = computed(() => game.value?.state === 'game')
  const isFinished = computed(() => game.value?.state === 'finished')

  function handle(m: WSMessage): void {
    switch (m.type) {
      case '_connected': connected.value = true; break
      case '_disconnected': connected.value = false; break
      case 'gameState': {
        const d = m.data
        const prevQuestionId = game.value?.currentQuestionId
        if (d.serverNow) {
          serverClockOffsetMs.value = new Date(d.serverNow).getTime() - Date.now()
        }
        game.value = {
          ...(game.value || {} as Game),
          code: d.code,
          name: d.name,
          state: d.state,
          questionState: d.questionState,
          currentQuestionId: d.currentQuestionId,
          questionStartedAt: d.questionStartedAt,
          questionTimeoutSeconds: d.questionTimeoutSeconds,
          scheduledAt: d.scheduledAt,
          questionIndex: d.questionIndex,
          totalQuestions: d.totalQuestions,
          leaderboardHidden: !!d.leaderboardHidden,
          mode: d.mode,
        }
        // A new question (or a re-activation) clears the lit-up names.
        if (d.currentQuestionId !== prevQuestionId || d.questionState === 'idle') {
          answeredUserIds.value = []
        }
        question.value = d.question || null
        if (d.leaderboard) leaderboard.value = d.leaderboard
        answers.value = d.answers || []
        break
      }
      case 'users':
        users.value = m.data ?? []
        break
      case 'answerAck':
        lastAnswerAck.value = m.data
        break
      case 'answeredSnapshot':
        if (m.data.questionId === game.value?.currentQuestionId) {
          answeredUserIds.value = m.data.userIds ?? []
        }
        break
      case 'playerAnswered':
        if (m.data.questionId === game.value?.currentQuestionId
          && !answeredUserIds.value.includes(m.data.userId)) {
          answeredUserIds.value = [...answeredUserIds.value, m.data.userId]
        }
        break
    }
  }

  // Board connections have no player token and never send anything; they reuse
  // the same message handler so the TV parses state exactly like a phone does.
  function ensureBoardWS(code: string): void {
    if (wsStarted.value) return
    onMessage(handle)
    wsConnectBoard(code)
    wsStarted.value = true
  }

  async function ensureWS(): Promise<void> {
    if (wsStarted.value) return
    const tok = localStorage.getItem('playerToken')
    if (!tok) return
    onMessage(handle)
    wsConnectPlayer(tok)
    wsStarted.value = true
  }

  async function loadMe(): Promise<void> {
    const tok = localStorage.getItem('playerToken')
    if (!tok) { me.value = null; game.value = null; return }
    try {
      const r = await playerApi.me()
      me.value = r.user
      game.value = r.game
    } catch {
      localStorage.removeItem('playerToken')
      me.value = null
      game.value = null
    }
  }

  function setMe(token: string, user: User): void {
    localStorage.setItem('playerToken', token)
    me.value = user
  }

  function updateMe(patch: Partial<User>): void {
    if (!me.value) return
    me.value = { ...me.value, ...patch }
  }

  function setGame(g: Game | null): void {
    game.value = g
  }

  function setUsers(list: User[]): void {
    users.value = list
  }

  function setLeaderboard(list: LeaderboardEntry[]): void {
    leaderboard.value = list
  }

  function resetWsState(): void {
    wsStarted.value = false
  }

  function logout(): void {
    localStorage.removeItem('playerToken')
    me.value = null
    game.value = null
    wsStarted.value = false
  }

  function setAdmin(token: string): void {
    localStorage.setItem('adminToken', token)
    isAdmin.value = true
  }

  function logoutAdmin(): void {
    localStorage.removeItem('adminToken')
    isAdmin.value = false
  }

  return {
    // state
    connected,
    me,
    game,
    question,
    leaderboard,
    users,
    answers,
    lastAnswerAck,
    answeredUserIds,
    wsStarted,
    isAdmin,
    serverClockOffsetMs,
    // getters
    isPlayer,
    isInGame,
    isFinished,
    // actions
    ensureWS,
    ensureBoardWS,
    loadMe,
    setMe,
    updateMe,
    setGame,
    setUsers,
    setLeaderboard,
    resetWsState,
    logout,
    setAdmin,
    logoutAdmin,
  }
})
