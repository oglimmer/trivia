// Vitest setup: jsdom does not always expose localStorage under Node 26+,
// so install a minimal in-memory shim and reset it before each test.
import { beforeEach } from 'vitest'

class MemoryStorage implements Storage {
  private store = new Map<string, string>()
  get length() { return this.store.size }
  clear() { this.store.clear() }
  getItem(k: string) { return this.store.has(k) ? this.store.get(k)! : null }
  setItem(k: string, v: string) { this.store.set(k, String(v)) }
  removeItem(k: string) { this.store.delete(k) }
  key(i: number) { return Array.from(this.store.keys())[i] ?? null }
}

const g = globalThis as unknown as { localStorage: Storage; sessionStorage: Storage }
g.localStorage = new MemoryStorage()
g.sessionStorage = new MemoryStorage()

beforeEach(() => {
  g.localStorage.clear()
  g.sessionStorage.clear()
})
