import type {
  AdminGameResponse,
  AdminGamesEntry,
  AISuggestResponse,
  Game,
  ImpersonateResponse,
  JoinResponse,
  LeaderboardEntry,
  MeResponse,
  Question,
  User,
} from '../types'

const BASE = '/api'

function headers(extra: Record<string, string> = {}): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json', ...extra }
  const playerToken = localStorage.getItem('playerToken')
  if (playerToken) h['X-Player-Token'] = playerToken
  const adminToken = localStorage.getItem('adminToken')
  if (adminToken) h['Authorization'] = 'Bearer ' + adminToken
  return h
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const r = await fetch(BASE + path, {
    method,
    headers: headers(),
    body: body == null ? undefined : JSON.stringify(body),
  })
  if (!r.ok) {
    let msg = r.statusText
    try { msg = (await r.json()).error || msg } catch {}
    throw new Error(msg)
  }
  if (r.status === 204) return null as T
  return r.json() as Promise<T>
}

export interface JoinBody { name: string; photoB64?: string }
export interface QuestionBody {
  text: string
  photoB64: string
  answerType: 'yesno' | 'choice' | 'number'
  options: string[]
  correct?: string | number
}
export interface CreateGameBody { code?: string; name: string; questionTimeoutSeconds?: number }
export interface UpdateSettingsBody { questionTimeoutSeconds?: number }
export interface AISuggestBody { hint: string; answerType: string; photoB64: string }

export const api = {
  // public
  getGame: (code: string) => req<Game>('GET', `/games/${code}`),
  joinGame: (code: string, body: JoinBody) => req<JoinResponse>('POST', `/games/${code}/join`, body),
  me: () => req<MeResponse>('GET', '/me'),
  updateMe: (body: Partial<User>) => req<User>('PUT', '/me', body),
  listUsers: (code: string) => req<User[]>('GET', `/games/${code}/users`),
  listQuestions: (code: string) => req<Question[]>('GET', `/games/${code}/questions`),
  putQuestion: (code: string, body: QuestionBody) => req<Question>('PUT', `/games/${code}/questions`, body),
  leaderboard: (code: string) => req<LeaderboardEntry[]>('GET', `/games/${code}/leaderboard`),
  aiSuggest: (body: AISuggestBody) => req<AISuggestResponse>('POST', '/ai/suggest', body),

  // admin
  adminLogin: (password: string) => req<{ token: string }>('POST', '/admin/login', { password }),
  adminGames: () => req<AdminGamesEntry[]>('GET', '/admin/games'),
  adminCreateGame: (body: CreateGameBody) => req<AdminGamesEntry>('POST', '/admin/games', body),
  adminGame: (code: string) => req<AdminGameResponse>('GET', `/admin/games/${code}`),
  adminDeleteGame: (code: string) => req<null>('DELETE', `/admin/games/${code}`),
  adminSetState: (code: string, state: string) => req<Game>('POST', `/admin/games/${code}/state`, { state }),
  adminUpdateSettings: (code: string, settings: UpdateSettingsBody) => req<Game>('PUT', `/admin/games/${code}/settings`, settings),
  adminActivate: (code: string, questionId: string | null) => req<Question | null>('POST', `/admin/games/${code}/activate`, { questionId }),
  adminReveal: (code: string) => req<null>('POST', `/admin/games/${code}/reveal`),
  adminNext: (code: string) => req<{ done?: boolean }>('POST', `/admin/games/${code}/next`),
  adminFinish: (code: string) => req<null>('POST', `/admin/games/${code}/finish`),
  adminDeleteUser: (code: string, userId: string) => req<null>('DELETE', `/admin/games/${code}/users/${userId}`),
  adminImpersonate: (code: string, userId: string) => req<ImpersonateResponse>('GET', `/admin/games/${code}/users/${userId}/impersonate`),
  adminDeleteQuestion: (code: string, questionId: string) => req<null>('DELETE', `/admin/games/${code}/questions/${questionId}`),
}
