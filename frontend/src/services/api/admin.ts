import { request } from './http'
import type {
  AdminAllUser,
  AdminGameResponse,
  AdminGamesEntry,
  Game,
  ImpersonateResponse,
  Question,
  QuestionResults,
} from '@/types'

export interface CreateGameBody { code?: string; name: string; questionTimeoutSeconds?: number; scheduledAt?: string | null }
export interface UpdateSettingsBody {
  questionTimeoutSeconds?: number
  // string -> set; null -> clear; omit -> leave unchanged.
  scheduledAt?: string | null
}

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
}
