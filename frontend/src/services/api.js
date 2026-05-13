const BASE = '/api'

function headers(extra = {}) {
  const h = { 'Content-Type': 'application/json', ...extra }
  const playerToken = localStorage.getItem('playerToken')
  if (playerToken) h['X-Player-Token'] = playerToken
  const adminToken = localStorage.getItem('adminToken')
  if (adminToken) h['Authorization'] = 'Bearer ' + adminToken
  return h
}

async function req(method, path, body) {
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
  if (r.status === 204) return null
  return r.json()
}

export const api = {
  // public
  getGame: (code) => req('GET', `/games/${code}`),
  joinGame: (code, body) => req('POST', `/games/${code}/join`, body),
  me: () => req('GET', '/me'),
  updateMe: (body) => req('PUT', '/me', body),
  listUsers: (code) => req('GET', `/games/${code}/users`),
  listQuestions: (code) => req('GET', `/games/${code}/questions`),
  putQuestion: (code, body) => req('PUT', `/games/${code}/questions`, body),
  leaderboard: (code) => req('GET', `/games/${code}/leaderboard`),
  aiSuggest: (body) => req('POST', '/ai/suggest', body),

  // admin
  adminLogin: (password) => req('POST', '/admin/login', { password }),
  adminGames: () => req('GET', '/admin/games'),
  adminCreateGame: (body) => req('POST', '/admin/games', body),
  adminGame: (code) => req('GET', `/admin/games/${code}`),
  adminDeleteGame: (code) => req('DELETE', `/admin/games/${code}`),
  adminSetState: (code, state) => req('POST', `/admin/games/${code}/state`, { state }),
  adminUpdateSettings: (code, settings) => req('PUT', `/admin/games/${code}/settings`, settings),
  adminActivate: (code, questionId) => req('POST', `/admin/games/${code}/activate`, { questionId }),
  adminReveal: (code) => req('POST', `/admin/games/${code}/reveal`),
  adminNext: (code) => req('POST', `/admin/games/${code}/next`),
  adminFinish: (code) => req('POST', `/admin/games/${code}/finish`),
  adminDeleteUser: (code, userId) => req('DELETE', `/admin/games/${code}/users/${userId}`),
  adminDeleteQuestion: (code, questionId) => req('DELETE', `/admin/games/${code}/questions/${questionId}`),
}
