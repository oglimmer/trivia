import { request } from './http'
import type {
  AdminAllUser,
  AdminGameResponse,
  AdminGamesEntry,
  Game,
  GameMode,
  ImpersonateResponse,
  Question,
  QuestionResults,
} from '@/types'

export interface CreateGameBody {
  code?: string
  name: string
  questionTimeoutSeconds?: number
  scheduledAt?: string | null
  // 'classic' (players write the questions) or 'poll' (host imports a
  // survey-derived set). Omit for classic.
  mode?: GameMode
}
export interface UpdateSettingsBody {
  questionTimeoutSeconds?: number
  // string -> set; null -> clear; omit -> leave unchanged.
  scheduledAt?: string | null
  hideLeaderboardTail?: boolean
}

// The pasted import payload: one entry per question, each with the top 5
// survey answers and how many people gave them.
export interface ImportAnswer { text: string; points: number }
export interface ImportQuestion { text: string; answers: ImportAnswer[] }
export interface ImportQuestionsBody { questions: ImportQuestion[] }

export const adminApi = {
  login: (password: string) => request<{ token: string }>('POST', '/admin/login', { password }),
  listGames: () => request<AdminGamesEntry[]>('GET', '/admin/games'),
  listAllUsers: () => request<AdminAllUser[]>('GET', '/admin/users'),
  createGame: (body: CreateGameBody) => request<AdminGamesEntry>('POST', '/admin/games', body),
  getGame: (code: string) => request<AdminGameResponse>('GET', `/admin/games/${code}`),
  // Reuses the public, finished-only results endpoint for per-question stats.
  // Vote tallies are fetched separately via votes() — they're admin-only.
  results: (code: string) => request<QuestionResults[]>('GET', `/games/${code}/results`),
  // Best-question vote counts keyed by questionId. Admin-only.
  votes: (code: string) => request<Record<string, number>>('GET', `/admin/games/${code}/votes`),
  deleteGame: (code: string) => request<null>('DELETE', `/admin/games/${code}`),
  setState: (code: string, state: string) => request<Game>('POST', `/admin/games/${code}/state`, { state }),
  updateSettings: (code: string, settings: UpdateSettingsBody) => request<Game>('PUT', `/admin/games/${code}/settings`, settings),
  activate: (code: string, questionId: string | null) => request<Question | null>('POST', `/admin/games/${code}/activate`, { questionId }),
  reveal: (code: string) => request<null>('POST', `/admin/games/${code}/reveal`),
  next: (code: string) => request<{ done?: boolean }>('POST', `/admin/games/${code}/next`),
  finish: (code: string) => request<null>('POST', `/admin/games/${code}/finish`),
  deleteUser: (code: string, userId: string) => request<null>('DELETE', `/admin/games/${code}/users/${userId}`),
  impersonate: (code: string, userId: string) => request<ImpersonateResponse>('GET', `/admin/games/${code}/users/${userId}/impersonate`),
  deleteQuestion: (code: string, questionId: string) => request<null>('DELETE', `/admin/games/${code}/questions/${questionId}`),
  importQuestions: (code: string, body: ImportQuestionsBody) =>
    request<{ imported: number; questions: Question[] }>('POST', `/admin/games/${code}/questions/import`, body),
  createQuestion: (code: string, body: ImportQuestion) =>
    request<Question>('POST', `/admin/games/${code}/questions`, body),
  updateQuestion: (code: string, questionId: string, body: ImportQuestion) =>
    request<Question>('PUT', `/admin/games/${code}/questions/${questionId}`, body),
  moveQuestion: (code: string, questionId: string, direction: 'up' | 'down') =>
    request<null>('POST', `/admin/games/${code}/questions/${questionId}/move`, { direction }),
}
