// Single WebSocket connection per page. Auto-reconnect with backoff.
// Emits typed messages to listeners.

import type { WSListener, WSMessage } from '../types'

const listeners = new Set<WSListener>()
let ws: WebSocket | null = null
let url: string | null = null
let backoff = 500
let retryTimer: ReturnType<typeof setTimeout> | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let lastRecvAt = 0
let paused = false

// A backgrounded tab / sleeping device can leave the WebSocket in readyState=1
// long after the underlying TCP connection died, so onclose never fires and
// state goes stale. Heartbeat catches this while the page is visible; the
// visibility/pageshow/online listeners catch it on wake.
const HEARTBEAT_MS = 20000
const STALE_MS = 30000

export function onMessage(fn: WSListener): () => void {
  listeners.add(fn)
  return () => { listeners.delete(fn) }
}

function emit(msg: WSMessage): void {
  for (const fn of listeners) fn(msg)
}

function startHeartbeat(): void {
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

function stopHeartbeat(): void {
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
}

function teardownSocket(): void {
  stopHeartbeat()
  if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
  if (ws) {
    const old = ws
    ws = null
    try { old.close() } catch {}
  }
}

function connect(): void {
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
    if (url && !paused) {
      retryTimer = setTimeout(connect, backoff)
      backoff = Math.min(backoff * 2, 5000)
    }
  }
  ws.onerror = () => { try { ws?.close() } catch {} }
  ws.onmessage = (ev: MessageEvent<string>) => {
    lastRecvAt = Date.now()
    try {
      const msg = JSON.parse(ev.data) as WSMessage
      if (msg && msg.type === 'pong') return
      emit(msg)
    } catch {}
  }
}

function reconnectNow(): void {
  backoff = 500
  teardownSocket()
  connect()
}

export function wsConnectPlayer(token: string): void {
  url = `/ws?token=${encodeURIComponent(token)}`
  reconnectNow()
}

export function wsConnectAdmin(adminToken: string, code: string): void {
  url = `/ws?role=admin&token=${encodeURIComponent(adminToken)}&code=${encodeURIComponent(code)}`
  reconnectNow()
}

export function wsSend(type: string, data: unknown): void {
  if (ws && ws.readyState === 1) {
    ws.send(JSON.stringify({ type, data }))
  }
}

export function disconnect(): void {
  url = null
  teardownSocket()
}

function onWake(): void {
  if (!url) return
  paused = false
  reconnectNow()
}

// Hide → close the socket so the server marks the player offline immediately
// instead of waiting for the 75s read deadline. The close frame goes out
// synchronously before the OS suspends the page on mobile.
function onHide(): void {
  if (!url) return
  paused = true
  teardownSocket()
}

if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) onHide()
    else onWake()
  })
  window.addEventListener('pagehide', onHide)
  window.addEventListener('pageshow', (e) => {
    if (e.persisted) onWake()
  })
  window.addEventListener('online', onWake)
}
