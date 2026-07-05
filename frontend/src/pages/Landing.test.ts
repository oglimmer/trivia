import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import Landing from '@/pages/Landing.vue'
import { createRouter, createWebHistory } from 'vue-router'

vi.mock('@/stores/game', () => ({
  useGameStore: vi.fn(() => ({
    loadMe: vi.fn(),
    me: null,
    game: null,
  })),
}))

vi.mock('@/services/api', () => ({
  playerApi: {
    getGame: vi.fn(),
  },
}))

const router = createRouter({
  history: createWebHistory(),
  routes: [],
})

describe('Landing.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the English language notice', () => {
    const wrapper = mount(Landing, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: true,
        },
      },
    })

    const badge = wrapper.find('.english-badge')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('Available only in English')
    expect(badge.attributes('role')).toBe('note')
  })
})
