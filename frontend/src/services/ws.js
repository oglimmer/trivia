// Single WebSocket connection per page. Auto-reconnect with backoff.
// Emits typed messages to listeners.

const listeners = new Set()
let ws = null
let url = null
let backoff = 500
let pingTimer = null

export function onMessage(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

function emit(msg) {
  for (const fn of listeners) fn(msg)
}

function connect() {
  if (!url) return
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const u = url.startsWith('ws') ? url : `${proto}//${location.host}${url}`
  ws = new WebSocket(u)
  ws.onopen = () => {
    backoff = 500
    emit({ type: '_connected' })
  }
  ws.onclose = () => {
    emit({ type: '_disconnected' })
    setTimeout(connect, backoff)
    backoff = Math.min(backoff * 2, 5000)
  }
  ws.onerror = () => { try { ws.close() } catch {} }
  ws.onmessage = (ev) => {
    try { emit(JSON.parse(ev.data)) } catch {}
  }
}

export function wsConnectPlayer(token) {
  url = `/ws?token=${encodeURIComponent(token)}`
  disconnect()
  connect()
}

export function wsConnectAdmin(adminToken, code) {
  url = `/ws?role=admin&token=${encodeURIComponent(adminToken)}&code=${encodeURIComponent(code)}`
  disconnect()
  connect()
}

export function wsSend(type, data) {
  if (ws && ws.readyState === 1) {
    ws.send(JSON.stringify({ type, data }))
  }
}

export function disconnect() {
  if (pingTimer) { clearInterval(pingTimer); pingTimer = null }
  if (ws) {
    const old = ws
    ws = null
    try { old.close() } catch {}
  }
}
