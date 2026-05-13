import { request } from './http'
import type {
  AISuggestResponse,
  Game,
  JoinResponse,
  LeaderboardEntry,
  MeResponse,
  Question,
  User,
} from '@/types'

export interface JoinBody { name: string; photoB64?: string }
export interface QuestionBody {
  text: string
  photoB64: string
  answerType: 'yesno' | 'choice' | 'number'
  options: string[]
  correct?: string | number
}
export interface AISuggestBody { hint: string; answerType: string; photoB64: string }

export const playerApi = {
  getGame: (code: string) => request<Game>('GET', `/games/${code}`),
  joinGame: (code: string, body: JoinBody) => request<JoinResponse>('POST', `/games/${code}/join`, body),
  me: () => request<MeResponse>('GET', '/me'),
  updateMe: (body: Partial<User>) => request<User>('PUT', '/me', body),
  listUsers: (code: string) => request<User[]>('GET', `/games/${code}/users`),
  listQuestions: (code: string) => request<Question[]>('GET', `/games/${code}/questions`),
  putQuestion: (code: string, body: QuestionBody) => request<Question>('PUT', `/games/${code}/questions`, body),
  leaderboard: (code: string) => request<LeaderboardEntry[]>('GET', `/games/${code}/leaderboard`),
  aiSuggest: (body: AISuggestBody) => request<AISuggestResponse>('POST', '/ai/suggest', body),
}
