// Single WebSocket connection per page. Auto-reconnect with backoff.
// Emits typed messages to listeners.

const listeners = new Set()
let ws = null
let url = null
let backoff = 500
let retryTimer = null
let heartbeatTimer = null
let lastRecvAt = 0

// A backgrounded tab / sleeping device can leave the WebSocket in readyState=1
// long after the underlying TCP connection died, so onclose never fires and
// state goes stale. Heartbeat catches this while the page is visible; the
// visibility/pageshow/online listeners catch it on wake.
const HEARTBEAT_MS = 20000
const STALE_MS = 30000

export function onMessage(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

function emit(msg) {
  for (const fn of listeners) fn(msg)
}

function startHeartbeat() {
  stopHeartbeat()
  heartbeatTimer = setInterval(() => {
    if (typeof document !== 'undefined' && document.hidden) return
    if (!ws) return
    if (lastRecvAt && Date.now() - lastRecvAt > STALE_MS) {
      try { ws.close() } catch {}
      return
    }
    if (ws.readyState === 1) {
      try { ws.send(JSON.stringify({ type: 'ping' })) } catch {}
    }
  }, HEARTBEAT_MS)
}

function stopHeartbeat() {
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
}

function teardownSocket() {
  stopHeartbeat()
  if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
  if (ws) {
    const old = ws
    ws = null
    try { old.close() } catch {}
  }
}

function connect() {
  if (!url) return
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const u = url.startsWith('ws') ? url : `${proto}//${location.host}${url}`
  ws = new WebSocket(u)
  ws.onopen = () => {
    backoff = 500
    lastRecvAt = Date.now()
    startHeartbeat()
    emit({ type: '_connected' })
  }
  ws.onclose = () => {
    stopHeartbeat()
    emit({ type: '_disconnected' })
    if (url) {
      retryTimer = setTimeout(connect, backoff)
      backoff = Math.min(backoff * 2, 5000)
    }
  }
  ws.onerror = () => { try { ws.close() } catch {} }
  ws.onmessage = (ev) => {
    lastRecvAt = Date.now()
    try {
      const msg = JSON.parse(ev.data)
      if (msg && msg.type === 'pong') return
      emit(msg)
    } catch {}
  }
}

function reconnectNow() {
  backoff = 500
  teardownSocket()
  connect()
}

export function wsConnectPlayer(token) {
  url = `/ws?token=${encodeURIComponent(token)}`
  reconnectNow()
}

export function wsConnectAdmin(adminToken, code) {
  url = `/ws?role=admin&token=${encodeURIComponent(adminToken)}&code=${encodeURIComponent(code)}`
  reconnectNow()
}

export function wsSend(type, data) {
  if (ws && ws.readyState === 1) {
    ws.send(JSON.stringify({ type, data }))
  }
}

export function disconnect() {
  url = null
  teardownSocket()
}

function onWake() {
  if (!url) return
  reconnectNow()
}

if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) onWake()
  })
  window.addEventListener('pageshow', (e) => {
    if (e.persisted) onWake()
  })
  window.addEventListener('online', onWake)
}
