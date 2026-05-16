<template>
  <AppHeader @edit-profile="editing = true" />

  <RouterView v-slot="{ Component }">
    <transition name="fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </RouterView>

  <AppFooter />

  <ConfirmDialog />
  <ProfileDialog :open="editing" @close="editing = false" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { RouterView } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import AppFooter from '@/components/AppFooter.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ProfileDialog from '@/components/ProfileDialog.vue'

const editing = ref(false)
</script>
