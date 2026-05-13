import { request } from './http'
import type {
  AdminGameResponse,
  AdminGamesEntry,
  Game,
  ImpersonateResponse,
  Question,
} from '@/types'

export interface CreateGameBody { code?: string; name: string; questionTimeoutSeconds?: number }
export interface UpdateSettingsBody { questionTimeoutSeconds?: number }

export const adminApi = {
  login: (password: string) => request<{ token: string }>('POST', '/admin/login', { password }),
  listGames: () => request<AdminGamesEntry[]>('GET', '/admin/games'),
  createGame: (body: CreateGameBody) => request<AdminGamesEntry>('POST', '/admin/games', body),
  getGame: (code: string) => request<AdminGameResponse>('GET', `/admin/games/${code}`),
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
