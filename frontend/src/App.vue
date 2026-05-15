<template>
  <AppHeader @edit-profile="editing = true" />

  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>

  <footer class="foot">
    <div class="foot__tagline">
      <span class="foot__author">Created by Oli Zimpasser</span>
      <span class="foot__sepInline" aria-hidden="true">·</span>
      <span class="foot__links">
        <a class="foot__link" href="https://github.com/oglimmer/trivia" target="_blank" rel="noopener noreferrer" aria-label="GitHub repository">
          <svg class="foot__ghIcon" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" focusable="false">
            <path fill="currentColor" fill-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.19 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
          </svg>
          <span>GitHub</span>
        </a>
        <span class="foot__dot" aria-hidden="true">·</span>
        <a class="foot__link" href="https://github.com/oglimmer/trivia/blob/master/LICENSE" target="_blank" rel="noopener noreferrer">MIT License</a>
      </span>
    </div>
    <div class="foot__legal">
      <RouterLink class="foot__link" to="/imprint">Imprint</RouterLink>
      <span class="foot__dot" aria-hidden="true">·</span>
      <RouterLink class="foot__link" to="/privacy">Privacy</RouterLink>
      <span class="foot__dot" aria-hidden="true">·</span>
      <RouterLink class="foot__link" to="/terms">Terms</RouterLink>
      <span class="foot__dot" aria-hidden="true">·</span>
      <RouterLink class="foot__link" to="/developers">Developers</RouterLink>
      <span class="foot__dot" aria-hidden="true">·</span>
      <button
        type="button"
        class="foot__infoBtn"
        aria-label="Show build info"
        @click="openBuildInfo"
      >
        <svg class="foot__infoIcon" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" focusable="false">
          <path fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"
                d="M8 1.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13z M8 7v4.5 M8 4.6v.1" />
        </svg>
      </button>
    </div>
  </footer>

  <ConfirmDialog />
  <ProfileDialog :open="editing" @close="editing = false" />
  <BuildInfoDialog :open="buildInfoOpen" @close="buildInfoOpen = false" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { RouterView, RouterLink } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ProfileDialog from '@/components/ProfileDialog.vue'
import BuildInfoDialog from '@/components/BuildInfoDialog.vue'
import { useBuildInfo } from '@/composables/useBuildInfo'

const editing = ref(false)
const buildInfoOpen = ref(false)

const { load } = useBuildInfo()

function openBuildInfo() {
  load()
  buildInfoOpen.value = true
}
</script>
