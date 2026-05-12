<template>
  <main class="stack">
    <div class="card stack">
      <h1>Join {{ code }}</h1>
      <p class="muted">Tell us who you are. Your photo shows up on leaderboards.</p>

      <label>Your name</label>
      <input v-model="name" placeholder="e.g. Sam" maxlength="40" />

      <label>Your photo</label>
      <PhotoPicker v-model="photo" />

      <button class="btn-primary" :disabled="!canSubmit || loading" @click="submit">
        {{ loading ? 'Joining…' : 'Join game' }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </div>
  </main>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import PhotoPicker from '../components/PhotoPicker.vue'
import { api } from '../services/api.js'
import { useGameStore } from '../stores/game.js'

const props = defineProps({ code: String })
const router = useRouter()
const store = useGameStore()

const name = ref('')
const photo = ref('')
const loading = ref(false)
const err = ref('')

const canSubmit = computed(() => name.value.trim().length > 0 && photo.value.length > 0)

async function submit() {
  err.value = ''
  loading.value = true
  try {
    const r = await api.joinGame(props.code, { name: name.value.trim(), photoB64: photo.value })
    store.setMe(r.token, { id: r.userId, name: name.value.trim(), gameId: r.gameId })
    router.push(`/g/${props.code}/setup`)
  } catch (e) {
    err.value = e.message || 'Could not join'
  } finally {
    loading.value = false
  }
}
</script>
