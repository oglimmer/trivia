<template>
  <AppHeader @edit-profile="editing = true" />

  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>

  <footer class="foot">
    <div class="foot__tagline">made for game night ★ no big tech, just friends</div>
    <div class="foot__versions" aria-label="Build information">
      <span class="foot__ver"><span class="foot__verLabel">frontend</span> v{{ frontend.version }} · {{ frontend.gitCommit }} · {{ frontend.buildTime }}</span>
      <span class="foot__sep">·</span>
      <span class="foot__ver"><span class="foot__verLabel">backend</span> {{ backendLine }}</span>
    </div>
  </footer>

  <ConfirmDialog />
  <ProfileDialog :open="editing" @close="editing = false" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterView } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ProfileDialog from '@/components/ProfileDialog.vue'
import { useBuildInfo } from '@/composables/useBuildInfo'

const editing = ref(false)

const { frontend, backend, load } = useBuildInfo()
onMounted(() => { load() })

const backendLine = computed(() => {
  if (!backend.value) return '…'
  return `v${backend.value.version} · ${backend.value.gitCommit} · ${backend.value.buildTime}`
})
</script>
