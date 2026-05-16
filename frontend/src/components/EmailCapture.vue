<template>
  <div class="email-offer stack">
    <p class="email-offer__lead">
      <strong>Got a minute?</strong> The game won't start for a while. Drop us your email
      and we'll send a one-click link so you can rejoin from any device.
    </p>
    <label for="locked-email" class="sr-only">Email</label>
    <div class="row" style="gap: 8px;">
      <input
        id="locked-email"
        v-model="email"
        type="email"
        placeholder="you@example.com"
        maxlength="120"
        autocomplete="email"
        inputmode="email"
        style="flex: 1;"
      />
      <button
        class="btn-primary"
        :disabled="!valid || saving"
        @click="save"
      >
        {{ saving ? '…' : 'Send link' }}
      </button>
    </div>
    <div v-if="err" class="error">{{ err }}</div>
    <div v-if="sent" class="email-offer__sent">Link sent! Check your inbox.</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { playerApi } from '@/services/api'
import { useGameStore } from '@/stores/game'
import { errMsg } from '@/composables/errMsg'

const store = useGameStore()

const email = ref('')
const saving = ref(false)
const sent = ref(false)
const err = ref('')

const valid = computed(() => /.+@.+\..+/.test(email.value.trim()))

async function save() {
  err.value = ''
  saving.value = true
  try {
    const trimmed = email.value.trim()
    await playerApi.updateMe({
      name: store.me?.name || '',
      photoImageId: store.me?.photoImageId || '',
      email: trimmed,
    })
    store.updateMe({ email: trimmed })
    sent.value = true
  } catch (e) {
    err.value = errMsg(e, 'Could not save email')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.email-offer {
  margin-top: 8px;
  padding: 14px;
  background: var(--paper, #fff);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-sm);
  box-shadow: 3px 3px 0 var(--ink);
  text-align: left;
  width: 100%;
}
.email-offer__lead {
  margin: 0 0 8px;
  line-height: 1.35;
}
.email-offer__sent {
  font-weight: 800;
  color: var(--ink);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
