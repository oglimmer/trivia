import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { WSMessage } from '@/types'
import {
  disconnect,
  onMessage,
  wsConnectAdmin,
  wsConnectPlayer,
  wsSend,
} from './ws'

// Minimal hand-rolled WebSocket double. The real one fires events
// asynchronously; we drive transitions synchronously from tests.
class FakeWS {
  static instances: FakeWS[] = []
  static reset() { FakeWS.instances = [] }
  static last(): FakeWS { return FakeWS.instances[FakeWS.instances.length - 1] }

  url: string
  readyState = 0
  sent: string[] = []
  closed = false
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeWS.instances.push(this)
  }

  send(data: string) { this.sent.push(data) }
  close() {
    if (this.closed) return
    this.closed = true
    this.readyState = 3
    this.onclose?.()
  }
  // Test helpers
  simulateOpen() { this.readyState = 1; this.onopen?.() }
  simulateMessage(msg: unknown) { this.onmessage?.({ data: JSON.stringify(msg) }) }
  simulateRawMessage(raw: string) { this.onmessage?.({ data: raw }) }
}

describe('ws service', () => {
  let received: WSMessage[]
  let unsubscribe: () => void

  beforeEach(() => {
    FakeWS.reset()
    vi.stubGlobal('WebSocket', FakeWS as unknown as typeof WebSocket)
    vi.useFakeTimers()
    received = []
    unsubscribe = onMessage((m) => { received.push(m) })
  })

  afterEach(() => {
    disconnect()
    unsubscribe()
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('wsConnectPlayer opens a /ws URL with the player token', () => {
    wsConnectPlayer('p tok/&')
    const fake = FakeWS.last()
    expect(fake.url).toMatch(/^wss?:\/\/.+\/ws\?token=p%20tok%2F%26$/)
  })

  it('wsConnectAdmin includes role, token and code params', () => {
    wsConnectAdmin('a-tok', 'ABCD')
    const fake = FakeWS.last()
    expect(fake.url).toContain('role=admin')
    expect(fake.url).toContain('token=a-tok')
    expect(fake.url).toContain('code=ABCD')
  })

  it('emits _connected on open', () => {
    wsConnectPlayer('t')
    FakeWS.last().simulateOpen()
    expect(received).toEqual([{ type: '_connected' }])
  })

  it('parses and emits incoming messages', () => {
    wsConnectPlayer('t')
    const ws = FakeWS.last()
    ws.simulateOpen()
    ws.simulateMessage({ type: 'users', data: [{ id: 'u1', name: 'A' }] })
    expect(received[1]).toEqual({ type: 'users', data: [{ id: 'u1', name: 'A' }] })
  })

  it('filters out pong messages', () => {
    wsConnectPlayer('t')
    const ws = FakeWS.last()
    ws.simulateOpen()
    ws.simulateMessage({ type: 'pong' })
    expect(received).toEqual([{ type: '_connected' }])
  })

  it('ignores malformed JSON without throwing', () => {
    wsConnectPlayer('t')
    const ws = FakeWS.last()
    ws.simulateOpen()
    expect(() => ws.simulateRawMessage('not-json')).not.toThrow()
    expect(received).toEqual([{ type: '_connected' }])
  })

  it('emits _disconnected on close and schedules a reconnect', () => {
    wsConnectPlayer('t')
    const first = FakeWS.last()
    first.simulateOpen()
    first.close()
    expect(received).toEqual([{ type: '_connected' }, { type: '_disconnected' }])
    expect(FakeWS.instances).toHaveLength(1)
    vi.advanceTimersByTime(500)
    expect(FakeWS.instances).toHaveLength(2)
  })

  it('reconnect backoff doubles up to 5s', () => {
    wsConnectPlayer('t')
    // Round 1: close → 500ms
    FakeWS.last().close()
    expect(FakeWS.instances).toHaveLength(1)
    vi.advanceTimersByTime(499)
    expect(FakeWS.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWS.instances).toHaveLength(2)
    // Round 2: 1000ms
    FakeWS.last().close()
    vi.advanceTimersByTime(999)
    expect(FakeWS.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(FakeWS.instances).toHaveLength(3)
    // Round 3: 2000ms
    FakeWS.last().close()
    vi.advanceTimersByTime(2000)
    expect(FakeWS.instances).toHaveLength(4)
  })

  it('disconnect() stops the reconnect loop', () => {
    wsConnectPlayer('t')
    FakeWS.last().close()
    disconnect()
    vi.advanceTimersByTime(10_000)
    expect(FakeWS.instances).toHaveLength(1)
  })

  it('wsSend forwards JSON when the socket is open', () => {
    wsConnectPlayer('t')
    const ws = FakeWS.last()
    ws.simulateOpen()
    wsSend('answer', { id: 'q1', value: 4 })
    expect(ws.sent).toEqual(['{"type":"answer","data":{"id":"q1","value":4}}'])
  })

  it('wsSend is a no-op when the socket is not open', () => {
    wsConnectPlayer('t')
    // Never simulate open — readyState stays 0.
    wsSend('answer', { id: 'q1' })
    expect(FakeWS.last().sent).toEqual([])
  })

  it('onMessage returns an unsubscribe handle', () => {
    const local: WSMessage[] = []
    const off = onMessage((m) => { local.push(m) })
    wsConnectPlayer('t')
    FakeWS.last().simulateOpen()
    expect(local).toHaveLength(1)
    off()
    FakeWS.last().simulateMessage({ type: 'users', data: [] })
    expect(local).toHaveLength(1)
  })
})
