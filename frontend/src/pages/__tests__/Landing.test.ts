import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import Landing from '@/pages/Landing.vue'

vi.mock('@/stores/game', () => ({
  useGameStore: () => ({
    me: null,
    game: null,
    loadMe: vi.fn(),
  }),
}))

describe('Landing page', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(async () => {
    const pinia = createPinia()
    setActivePinia(pinia)

    router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/', component: Landing }],
    })
    router.push('/')
    await router.isReady()
  })

  it('shows the English‑only notice', () => {
    const wrapper = mount(Landing, {
      global: {
        plugins: [createPinia(), router],
        stubs: { RouterLink: true },
      },
    })

    const notice = wrapper.find('[role="alert"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('English')
  })
})
