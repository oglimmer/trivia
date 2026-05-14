export interface BackendBuildInfo {
  name: string
  version: string
  gitCommit: string
  buildTime: string
}

export type AnswerType = 'yesno' | 'choice' | 'number'
export type GameState = 'setup' | 'game' | 'finished'
export type QuestionState = 'idle' | 'active' | 'revealed'

export interface User {
  id: string
  name: string
  photoB64?: string
  gameId?: string
}

export interface Game {
  code: string
  name: string
  state: GameState
  questionState?: QuestionState
  currentQuestionId?: string | null
  questionStartedAt?: string | null
  questionTimeoutSeconds?: number
}

export interface Question {
  id: string
  userId: string
  text: string
  photoB64: string
  answerType: AnswerType
  options?: string[]
  correct: string | number
}

export interface Answer {
  id: string
  userId: string
  questionId: string
  value: unknown
  isCorrect: boolean
  points: number
  responseMs: number
}

export interface LeaderboardEntry {
  userId: string
  userName: string
  photoB64?: string
  points: number
}

export interface AnswerAck {
  questionId: string
  responseMs: number
  isCorrect?: boolean
  points?: number
}

export interface MeResponse {
  user: User
  game: Game | null
}

export interface JoinResponse {
  token: string
  userId: string
  gameId: string
}

export interface AdminGameResponse {
  game: Game
  users: User[]
  questions: Question[]
  online?: string[]
}

export interface AdminAllUser {
  id: string
  gameId: string
  gameCode: string
  gameName: string
  name: string
  photoB64?: string
  createdAt: string
}

export interface AdminGamesEntry {
  id: string
  code: string
  name: string
  state: GameState
  playerCount?: number
  onlineCount?: number
  questionTimeoutSeconds?: number
}

export interface ImpersonateResponse {
  token: string
}

export interface AISuggestResponse {
  text?: string
  options?: string[]
  correct?: string | number
}

export interface GameStateMsg {
  code: string
  name: string
  state: GameState
  questionState?: QuestionState
  currentQuestionId?: string | null
  questionStartedAt?: string | null
  questionTimeoutSeconds?: number
  question?: Question | null
  leaderboard?: LeaderboardEntry[]
  answers?: Answer[]
  serverNow?: string
}

export interface PlayerAnsweredMsg {
  userId: string
}

export interface PresenceMsg {
  online?: string[]
}

export type WSMessage =
  | { type: '_connected' }
  | { type: '_disconnected' }
  | { type: 'pong' }
  | { type: 'gameState'; data: GameStateMsg }
  | { type: 'users'; data: User[] }
  | { type: 'questionsAdmin'; data: Question[] }
  | { type: 'playerAnswered'; data: PlayerAnsweredMsg }
  | { type: 'presence'; data: PresenceMsg }
  | { type: 'answerAck'; data: AnswerAck }

export type WSListener = (msg: WSMessage) => void
