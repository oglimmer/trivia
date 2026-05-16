import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from './http'

interface FetchCall {
  url: string
  init: RequestInit
}

function mockFetch(response: { status: number; body?: unknown; bodyText?: string }) {
  const calls: FetchCall[] = []
  const fn = vi.fn(async (url: string, init: RequestInit) => {
    calls.push({ url, init })
    const text = response.bodyText
      ?? (response.body === undefined ? '' : JSON.stringify(response.body))
    return new Response(text || null, {
      status: response.status,
      statusText: response.status === 401 ? 'Unauthorized' : 'OK',
    })
  })
  vi.stubGlobal('fetch', fn)
  return { calls, fn }
}

function headers(init: RequestInit): Record<string, string> {
  return init.headers as Record<string, string>
}

describe('http.request', () => {
  beforeEach(() => {
    // jsdom defaults to about:blank; give it a real path so location.replace assertions work.
    window.history.replaceState({}, '', '/')
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends JSON body and Content-Type header', async () => {
    const { calls } = mockFetch({ status: 200, body: { ok: true } })
    const out = await request<{ ok: boolean }>('POST', '/things', { a: 1 })
    expect(out).toEqual({ ok: true })
    expect(calls[0].url).toBe('/api/things')
    expect(calls[0].init.method).toBe('POST')
    expect(calls[0].init.body).toBe('{"a":1}')
    expect(headers(calls[0].init)['Content-Type']).toBe('application/json')
  })

  it('omits body when none is given', async () => {
    const { calls } = mockFetch({ status: 200, body: { ok: true } })
    await request('GET', '/things')
    expect(calls[0].init.body).toBeUndefined()
  })

  it('attaches X-Player-Token when set', async () => {
    localStorage.setItem('playerToken', 'p-tok')
    const { calls } = mockFetch({ status: 200, body: {} })
    await request('GET', '/me')
    expect(headers(calls[0].init)['X-Player-Token']).toBe('p-tok')
    expect(headers(calls[0].init).Authorization).toBeUndefined()
  })

  it('attaches Authorization Bearer when admin token set', async () => {
    localStorage.setItem('adminToken', 'a-tok')
    const { calls } = mockFetch({ status: 200, body: {} })
    await request('GET', '/admin/games')
    expect(headers(calls[0].init).Authorization).toBe('Bearer a-tok')
  })

  it('returns null for 204 responses', async () => {
    mockFetch({ status: 204 })
    const out = await request('DELETE', '/things/1')
    expect(out).toBeNull()
  })

  it('throws with server error message when present', async () => {
    mockFetch({ status: 400, body: { error: 'bad code' } })
    await expect(request('POST', '/things')).rejects.toThrow('bad code')
  })

  it('falls back to statusText when error body is not JSON', async () => {
    mockFetch({ status: 500, bodyText: 'kaboom' })
    await expect(request('GET', '/things')).rejects.toThrow('OK')
  })

  it('clears admin token and redirects on 401 from /admin/*', async () => {
    localStorage.setItem('adminToken', 'a-tok')
    window.history.replaceState({}, '', '/admin/games')
    const replace = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, pathname: '/admin/games', replace },
    })
    mockFetch({ status: 401, body: { error: 'expired' } })
    await expect(request('GET', '/admin/games')).rejects.toThrow('expired')
    expect(localStorage.getItem('adminToken')).toBeNull()
    expect(replace).toHaveBeenCalledWith('/admin')
  })

  it('clears player token and redirects on 401 from non-admin path', async () => {
    localStorage.setItem('playerToken', 'p-tok')
    const replace = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, pathname: '/game', replace },
    })
    mockFetch({ status: 401, body: { error: 'expired' } })
    await expect(request('GET', '/me')).rejects.toThrow()
    expect(localStorage.getItem('playerToken')).toBeNull()
    expect(replace).toHaveBeenCalledWith('/')
  })

  it('does not redirect on 401 when no matching token was sent', async () => {
    // No tokens in storage at all.
    const replace = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, pathname: '/', replace },
    })
    mockFetch({ status: 401, body: { error: 'nope' } })
    await expect(request('GET', '/me')).rejects.toThrow()
    expect(replace).not.toHaveBeenCalled()
  })
})
