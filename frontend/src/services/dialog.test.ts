import { afterEach, describe, expect, it } from 'vitest'
import { confirm, dialogState, resolveDialog } from './dialog'

describe('dialog service', () => {
  afterEach(() => {
    // Make sure no dangling dialog leaks into the next test.
    resolveDialog(false)
  })

  it('starts with no active dialog', () => {
    expect(dialogState.current).toBeNull()
  })

  it('exposes the current dialog while open and clears it on resolve', async () => {
    const p = confirm({ title: 'Continue?', message: 'sure?' })
    expect(dialogState.current?.title).toBe('Continue?')
    expect(dialogState.current?.message).toBe('sure?')
    resolveDialog(true)
    await expect(p).resolves.toBe(true)
    expect(dialogState.current).toBeNull()
  })

  it('applies sensible defaults to omitted fields', () => {
    confirm({})
    const d = dialogState.current!
    expect(d.title).toBe('Are you sure?')
    expect(d.confirmLabel).toBe('Confirm')
    expect(d.cancelLabel).toBe('Cancel')
    expect(d.tone).toBe('primary')
    expect(d.icon).toBe('❓')
  })

  it('uses a warning icon by default when tone is danger', () => {
    confirm({ tone: 'danger' })
    expect(dialogState.current?.icon).toBe('⚠️')
    expect(dialogState.current?.tone).toBe('danger')
  })

  it('honors an explicit custom icon over the tone default', () => {
    confirm({ tone: 'danger', icon: '🔥' })
    expect(dialogState.current?.icon).toBe('🔥')
  })

  it('resolveDialog is a no-op when nothing is open', () => {
    expect(() => resolveDialog(true)).not.toThrow()
    expect(dialogState.current).toBeNull()
  })

  it('supports sequential dialogs', async () => {
    const p1 = confirm({ title: 'A' })
    resolveDialog(true)
    await expect(p1).resolves.toBe(true)
    const p2 = confirm({ title: 'B' })
    expect(dialogState.current?.title).toBe('B')
    resolveDialog(false)
    await expect(p2).resolves.toBe(false)
  })
})
