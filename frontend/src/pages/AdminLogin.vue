<template>
  <main class="stack-lg" style="padding-top: 16px;">
    <section class="hero" style="background: var(--blue-2);">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>

      <span class="hero__eyebrow">Host mode</span>
      <h1 class="hero__title">Run the <em>show</em>.</h1>
      <p class="hero__subtitle">Sign in to create rooms and conduct the game.</p>
    </section>

    <div class="card stack">
      <label for="admin-pwd">Host password</label>
      <input
        id="admin-pwd"
        v-model="pwd"
        type="password"
        placeholder="••••••••"
        @keyup.enter="login"
        autocomplete="current-password"
      />
      <button class="btn-blue btn-lg btn-block" :disabled="!pwd || loading" @click="login">
        {{ loading ? 'Signing in…' : 'Sign in →' }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <RouterLink to="/" class="btn-ghost btn-block">← Back to player join</RouterLink>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../services/api'
import { useGameStore } from '../stores/game'

const pwd = ref('')
const loading = ref(false)
const err = ref('')
const router = useRouter()
const store = useGameStore()

onMounted(() => {
  if (localStorage.getItem('adminToken')) router.replace('/admin/games')
})

async function login() {
  err.value = ''
  loading.value = true
  try {
    const r = await api.adminLogin(pwd.value)
    store.setAdmin(r.token)
    router.push('/admin/games')
  } catch (e) {
    err.value = (e as Error).message || 'login failed'
  } finally {
    loading.value = false
  }
}
</script>
