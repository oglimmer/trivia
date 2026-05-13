import { reactive, readonly, type DeepReadonly } from 'vue'

export type DialogTone = 'primary' | 'danger'

export interface ConfirmOptions {
  title?: string
  message?: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: DialogTone
  icon?: string
}

export interface DialogEntry {
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  tone: DialogTone
  icon: string
  resolve: (v: boolean) => void
}

interface DialogState {
  current: DialogEntry | null
}

const state: DialogState = reactive({ current: null })

export const dialogState: DeepReadonly<DialogState> = readonly(state)

export function confirm(opts: ConfirmOptions = {}): Promise<boolean> {
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

export function resolveDialog(value: boolean): void {
  const c = state.current
  if (!c) return
  state.current = null
  c.resolve(value)
}
