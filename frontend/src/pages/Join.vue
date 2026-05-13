<template>
  <main class="stack-lg">
    <!-- Hero: explicit profile-setup framing -->
    <section class="profile-hero">
      <div class="profile-hero__sparkles" aria-hidden="true">
        <span class="profile-hero__sparkle s1">✦</span>
        <span class="profile-hero__sparkle s2">★</span>
        <span class="profile-hero__sparkle s3">✺</span>
      </div>
      <div class="profile-hero__eyebrow">Step 1 · Your profile</div>
      <h1 class="profile-hero__title">Make your player card</h1>
      <p class="profile-hero__sub">
        Pick a name &amp; selfie — this is <em>you</em> on the leaderboard.
        Your trivia question comes next.
      </p>
    </section>

    <!-- Live preview of the player card the user is building -->
    <section class="player-card" aria-label="Live preview of your player card">
      <div class="player-card__photo">
        <img v-if="photo" :src="photo" alt="" />
        <span v-else class="player-card__placeholder" aria-hidden="true">🙂</span>
      </div>
      <div :class="['player-card__name', !name.trim() && 'is-empty']">
        {{ name.trim() || 'Your name' }}
      </div>
      <div class="player-card__game">
        <span>Game</span>
        <span class="player-card__game-code">{{ code }}</span>
      </div>
    </section>

    <!-- Profile inputs -->
    <section class="card card--mint stack">
      <label for="player-name">Display name</label>
      <input
        id="player-name"
        v-model="name"
        placeholder="e.g. Sam"
        maxlength="40"
        autocomplete="given-name"
      />

      <label>Your selfie</label>
      <PhotoPicker v-model="photo" no-frame allow-random />

      <button
        class="btn-primary btn-lg btn-block"
        :disabled="!canSubmit || loading"
        @click="submit"
      >
        {{ loading ? 'Saving…' : 'Save my card →' }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </section>

    <!-- What's next: makes the two-step flow explicit -->
    <div class="next-hint">
      <span class="next-hint__num">2</span>
      <span><strong>Next up:</strong> write a trivia question for the game.</span>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import PhotoPicker from '@/components/PhotoPicker.vue'
import { playerApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'

const props = defineProps<{ code: string }>()
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
    const r = await playerApi.joinGame(props.code, { name: name.value.trim(), photoB64: photo.value })
    store.setMe(r.token, { id: r.userId, name: name.value.trim(), gameId: r.gameId })
    router.push(`/g/${props.code}/setup`)
  } catch (e) {
    err.value = errMsg(e, 'Could not join')
  } finally {
    loading.value = false
  }
}
</script>
