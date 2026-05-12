<template>
  <main class="stack" style="padding-top: 32px;">
    <div class="card stack">
      <h1>Admin</h1>
      <p class="muted">Enter the host password.</p>
      <input v-model="pwd" type="password" placeholder="••••••••" @keyup.enter="login" autocomplete="current-password" />
      <button class="btn-primary" :disabled="!pwd || loading" @click="login">
        {{ loading ? 'Signing in…' : 'Sign in' }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../services/api.js'
import { useGameStore } from '../stores/game.js'

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
    err.value = e.message || 'login failed'
  } finally {
    loading.value = false
  }
}
</script>
