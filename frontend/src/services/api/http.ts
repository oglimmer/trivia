const BASE = '/api'

interface AuthHeaders {
  headers: Record<string, string>
  usedAdmin: boolean
  usedPlayer: boolean
}

function buildHeaders(extra: Record<string, string> = {}): AuthHeaders {
  const h: Record<string, string> = { 'Content-Type': 'application/json', ...extra }
  const playerToken = localStorage.getItem('playerToken')
  if (playerToken) h['X-Player-Token'] = playerToken
  const adminToken = localStorage.getItem('adminToken')
  if (adminToken) h['Authorization'] = 'Bearer ' + adminToken
  return { headers: h, usedAdmin: !!adminToken, usedPlayer: !!playerToken }
}

function handleUnauthorized(path: string, usedAdmin: boolean, usedPlayer: boolean): void {
  // Admin endpoints live under /admin; a 401 there means the admin JWT expired
  // or was rejected. Player endpoints reject on a stale player token.
  const isAdminPath = path.startsWith('/admin')
  if (isAdminPath && usedAdmin) {
    localStorage.removeItem('adminToken')
    if (window.location.pathname !== '/admin') window.location.replace('/admin')
  } else if (!isAdminPath && usedPlayer) {
    localStorage.removeItem('playerToken')
    if (window.location.pathname !== '/') window.location.replace('/')
  }
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const { headers, usedAdmin, usedPlayer } = buildHeaders()
  const r = await fetch(BASE + path, {
    method,
    headers,
    body: body == null ? undefined : JSON.stringify(body),
  })
  if (!r.ok) {
    let msg = r.statusText
    try { msg = (await r.json()).error || msg } catch {}
    if (r.status === 401) handleUnauthorized(path, usedAdmin, usedPlayer)
    throw new Error(msg)
  }
  if (r.status === 204) return null as T
  return r.json() as Promise<T>
}
