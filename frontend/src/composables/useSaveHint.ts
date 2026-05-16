import { nextTick, onUnmounted, ref, type Ref } from 'vue'

// Shows a "scroll down to save" hint until the save button enters the viewport,
// then hides it automatically. The hint can also be dismissed explicitly via
// `dismiss()` (used when the user clicks the hint to scroll, or after save).
export function useSaveHint(targetRef: Ref<HTMLElement | null>) {
  const visible = ref(false)
  let observer: IntersectionObserver | null = null

  function disconnect() {
    observer?.disconnect()
    observer = null
  }

  function arm() {
    disconnect()
    visible.value = true
    nextTick(() => {
      const el = targetRef.value
      if (!el || typeof IntersectionObserver === 'undefined') return
      observer = new IntersectionObserver((entries) => {
        if (entries.some(e => e.isIntersecting)) {
          visible.value = false
          disconnect()
        }
      }, { threshold: 0.5 })
      observer.observe(el)
    })
  }

  function dismiss() {
    visible.value = false
    disconnect()
  }

  function scrollTo() {
    dismiss()
    targetRef.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  onUnmounted(disconnect)

  return { visible, arm, dismiss, scrollTo }
}
