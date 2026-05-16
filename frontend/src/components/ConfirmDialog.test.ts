import { afterEach, describe, expect, it } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ConfirmDialog from './ConfirmDialog.vue'
import { confirm, resolveDialog } from '@/services/dialog'

describe('ConfirmDialog', () => {
  afterEach(() => {
    // The component renders into a shared module-level dialog state; reset it
    // so the next test starts clean.
    resolveDialog(false)
  })

  it('renders nothing when no dialog is open', () => {
    const w = mount(ConfirmDialog)
    expect(w.find('[role="dialog"]').exists()).toBe(false)
  })

  it('renders title / message / labels when opened', async () => {
    const w = mount(ConfirmDialog)
    confirm({ title: 'Delete user?', message: 'This cannot be undone.', confirmLabel: 'Yes', cancelLabel: 'No' })
    await flushPromises()
    expect(w.find('.dialog__title').text()).toBe('Delete user?')
    expect(w.find('.dialog__msg').text()).toBe('This cannot be undone.')
    const btns = w.findAll('button')
    expect(btns[0].text()).toBe('No')
    expect(btns[1].text()).toBe('Yes')
  })

  it('confirm button resolves the promise with true', async () => {
    const w = mount(ConfirmDialog)
    const p = confirm({ title: 'Go?' })
    await flushPromises()
    await w.findAll('button')[1].trigger('click')
    await expect(p).resolves.toBe(true)
  })

  it('cancel button resolves the promise with false', async () => {
    const w = mount(ConfirmDialog)
    const p = confirm({ title: 'Go?' })
    await flushPromises()
    await w.findAll('button')[0].trigger('click')
    await expect(p).resolves.toBe(false)
  })

  it('uses the danger tone styling when requested', async () => {
    const w = mount(ConfirmDialog)
    confirm({ title: 'Drop table?', tone: 'danger' })
    await flushPromises()
    expect(w.find('.dialog').classes()).toContain('dialog--danger')
    expect(w.findAll('button')[1].classes()).toContain('btn-danger')
  })
})
