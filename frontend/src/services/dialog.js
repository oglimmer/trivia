import { reactive, readonly } from 'vue'

const state = reactive({ current: null })

export const dialogState = readonly(state)

export function confirm(opts = {}) {
  return new Promise((resolve) => {
    state.current = {
      title: opts.title || 'Are you sure?',
      message: opts.message || '',
      confirmLabel: opts.confirmLabel || 'Confirm',
      cancelLabel: opts.cancelLabel || 'Cancel',
      tone: opts.tone === 'danger' ? 'danger' : 'primary',
      icon: opts.icon || (opts.tone === 'danger' ? '⚠️' : '❓'),
      resolve,
    }
  })
}

export function resolveDialog(value) {
  const c = state.current
  if (!c) return
  state.current = null
  c.resolve(value)
}
