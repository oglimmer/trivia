import { nextTick, onUnmounted, watch, type Ref, type WatchSource } from 'vue'

// Locks body scroll while open, traps focus to `focusTarget`, restores focus
// on close. Shared by ConfirmDialog and ProfileDialog.
export function useModal(
  isOpen: WatchSource<boolean>,
  focusTarget: () => HTMLElement | null,
): void {
  let prevFocus: Element | null = null
  let prevOverflow = ''
  let active = false

  watch(isOpen, async (open) => {
    if (open) {
      prevFocus = document.activeElement
      prevOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      active = true
      await nextTick()
      focusTarget()?.focus()
    } else if (active) {
      document.body.style.overflow = prevOverflow
      if (prevFocus && 'focus' in prevFocus && typeof (prevFocus as HTMLElement).focus === 'function') {
        (prevFocus as HTMLElement).focus()
      }
      prevFocus = null
      active = false
    }
  })

  onUnmounted(() => {
    if (active) document.body.style.overflow = prevOverflow
  })
}

// Convenience helper: takes a Ref<boolean> directly.
export function useModalRef(openRef: Ref<boolean>, focusTarget: () => HTMLElement | null): void {
  useModal(() => openRef.value, focusTarget)
}
