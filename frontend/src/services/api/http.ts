const BASE = '/api'

function headers(extra: Record<string, string> = {}): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json', ...extra }
  const playerToken = localStorage.getItem('playerToken')
  if (playerToken) h['X-Player-Token'] = playerToken
  const adminToken = localStorage.getItem('adminToken')
  if (adminToken) h['Authorization'] = 'Bearer ' + adminToken
  return h
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
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
