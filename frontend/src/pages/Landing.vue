<template>
  <main class="stack-lg" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__sparkle s3" aria-hidden="true">★</span>

      <span class="hero__eyebrow">{{ t('heroEyebrow') }}</span>
      <h1 class="hero__title" v-html="t('heroMotto')"></h1>
      <p class="hero__subtitle">{{ t('heroSubtitle') }}</p>
    </section>

    <section class="card stack">
      <label for="join-code">{{ t('gameCodeLabel') }}</label>
      <input
        id="join-code"
        ref="codeInput"
        v-model="code"
        @keyup.enter="join"
        :placeholder="t('gameCodePlaceholder')"
        maxlength="8"
        autocapitalize="off"
        autocomplete="off"
        spellcheck="false"
        class="code-input"
      />
      <button class="btn-primary btn-lg btn-block" :disabled="!code || loading" @click="join">
        {{ loading ? t('loadingButton') : t('continueButton') }}
      </button>
      <div v-if="err" class="error">{{ err }}</div>
    </section>

    <div class="row" style="justify-content: center; gap: 18px;">
      <span class="muted" style="font-size: .85rem;">{{ t('hosting') }}</span>
      <RouterLink to="/admin" class="btn-link">{{ t('openAdmin') }}</RouterLink>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { playerApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'
import { useI18n } from '@/composables/useI18n'
import type { GameState } from '@/types'

const router = useRouter()
const code = ref('')
const loading = ref(false)
const err = ref('')
const store = useGameStore()
const codeInput = ref<HTMLInputElement | null>(null)
const { t } = useI18n()

onMounted(async () => {
  codeInput.value?.focus()
  await store.loadMe()
  if (store.me && store.game) {
    routeForState(store.game.code, store.game.state)
  }
})

function routeForState(c: string, state: GameState) {
  if (state === 'setup') router.replace(`/g/${c}/setup`)
  else if (state === 'game') router.replace(`/g/${c}/play`)
  else if (state === 'finished') router.replace(`/g/${c}/results`)
}

async function join() {
  err.value = ''
  loading.value = true
  try {
    const c = code.value.trim().toLowerCase()
    const g = await playerApi.getGame(c)
    if (g.state === 'finished') {
      router.push(`/g/${c}/results`)
    } else {
      router.push(`/g/${c}/join`)
    }
  } catch (e) {
    err.value = errMsg(e, 'No game with that code')
  } finally {
    loading.value = false
  }
}
</script>
