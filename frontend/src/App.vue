<template>
  <AppHeader v-if="!fullscreen" @edit-profile="editing = true" />

  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>

  <AppFooter v-if="!fullscreen" />

  <ConfirmDialog />
  <ProfileDialog :open="editing" @close="editing = false" />
</template>

<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import AppFooter from '@/components/AppFooter.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ProfileDialog from '@/components/ProfileDialog.vue'

const editing = ref(false)

// The TV board wants the whole screen: no header, no footer, no 760px column.
const route = useRoute()
const fullscreen = computed(() => route.meta.fullscreen === true)
watchEffect(() => {
  document.documentElement.classList.toggle('app--fullscreen', fullscreen.value)
})
</script>
