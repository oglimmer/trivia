import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { useModalRef } from './useModal'

function harness() {
  const open = ref(false)
  const focusable = document.createElement('button')
  focusable.textContent = 'inside'
  // Has to be in the DOM so .focus() actually moves activeElement.
  document.body.appendChild(focusable)

  const Host = defineComponent({
    setup() {
      useModalRef(open, () => focusable)
      return {}
    },
    template: '<div />',
  })
  const wrapper = mount(Host, { attachTo: document.body })
  return { open, focusable, wrapper }
}

describe('useModal', () => {
  afterEach(() => {
    document.body.style.overflow = ''
    document.body.innerHTML = ''
  })

  it('does nothing while closed', () => {
    harness()
    expect(document.body.style.overflow).toBe('')
  })

  it('locks body scroll and focuses target on open', async () => {
    const { open, focusable } = harness()
    open.value = true
    await flushPromises()
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.activeElement).toBe(focusable)
  })

  it('restores body scroll and previous focus on close', async () => {
    const previouslyFocused = document.createElement('button')
    previouslyFocused.textContent = 'outside'
    document.body.appendChild(previouslyFocused)
    previouslyFocused.focus()
    expect(document.activeElement).toBe(previouslyFocused)

    document.body.style.overflow = 'auto'
    const { open } = harness()
    open.value = true
    await flushPromises()
    expect(document.body.style.overflow).toBe('hidden')

    open.value = false
    await flushPromises()
    expect(document.body.style.overflow).toBe('auto')
    expect(document.activeElement).toBe(previouslyFocused)
  })

  it('restores scroll on unmount if still open', async () => {
    document.body.style.overflow = 'auto'
    const { open, wrapper } = harness()
    open.value = true
    await flushPromises()
    expect(document.body.style.overflow).toBe('hidden')
    wrapper.unmount()
    expect(document.body.style.overflow).toBe('auto')
  })
})
