import { request } from './http'
import type {
  AISuggestResponse,
  Game,
  JoinResponse,
  LeaderboardEntry,
  MeResponse,
  Question,
  QuestionResults,
  User,
} from '@/types'

export interface JoinBody { name: string; photoImageId?: string; email?: string }
export interface QuestionBody {
  text: string
  photoImageId: string
  answerType: 'yesno' | 'choice' | 'number'
  options: string[]
  correct?: string | number
}
export interface AISuggestBody { hint: string; answerType: string; photoImageId?: string }

export const playerApi = {
  getGame: (code: string) => request<Game>('GET', `/games/${code}`),
  joinGame: (code: string, body: JoinBody) => request<JoinResponse>('POST', `/games/${code}/join`, body),
  me: () => request<MeResponse>('GET', '/me'),
  updateMe: (body: Partial<User>) => request<User>('PUT', '/me', body),
  listUsers: (code: string) => request<User[]>('GET', `/games/${code}/users`),
  listQuestions: (code: string) => request<Question[]>('GET', `/games/${code}/questions`),
  putQuestion: (code: string, body: QuestionBody) => request<Question>('PUT', `/games/${code}/questions`, body),
  leaderboard: (code: string) => request<LeaderboardEntry[]>('GET', `/games/${code}/leaderboard`),
  results: (code: string) => request<QuestionResults[]>('GET', `/games/${code}/results`),
  myVote: (code: string) => request<{ questionId: string }>('GET', `/games/${code}/myvote`),
  castVote: (code: string, questionId: string) =>
    request<{ questionId: string; cast: boolean }>('POST', `/games/${code}/vote`, { questionId }),
  aiSuggest: (body: AISuggestBody) => request<AISuggestResponse>('POST', '/ai/suggest', body),
}
