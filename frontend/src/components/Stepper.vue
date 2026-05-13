<template>
  <div class="stepper" :aria-label="`Step ${current} of ${steps.length}`">
    <template v-for="(s, i) in steps" :key="s.n">
      <div :class="['stepper__step', stateOf(s.n)]">
        <div class="stepper__dot">{{ stateOf(s.n) === 'done' ? '✓' : s.n }}</div>
        <div class="stepper__label">{{ s.label }}</div>
      </div>
      <div
        v-if="i < steps.length - 1"
        :class="['stepper__bar', s.n < current && 'done']"
      ></div>
    </template>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ current: number }>()

const steps = [
  { n: 1, label: 'Photo' },
  { n: 2, label: 'AI?' },
  { n: 3, label: 'Details' },
]

function stateOf(n: number): 'done' | 'active' | '' {
  if (n < props.current) return 'done'
  if (n === props.current) return 'active'
  return ''
}
</script>
