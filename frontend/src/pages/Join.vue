<template>
  <main class="stack-lg">
    <div class="card card--yellow stack">
      <span class="tag tag--pink" style="align-self: flex-start;">Game · {{ code }}</span>
      <h1>Who's playing?</h1>
      <p class="muted">Your name &amp; photo will pop up on the leaderboard.</p>
    </div>

    <div class="card stack">
      <label for="player-name">Your name</label>
      <input id="player-name" v-model="name" placeholder="e.g. Sam" maxlength="40" />

      <label>Your photo</label>
      <PhotoPicker v-model="photo" />

      <button class="btn-primary btn-lg btn-block" :disabled="!canSubmit || loading" @click="submit">
        {{ loading ? 'Joining…' : "Let's go →" }}
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
