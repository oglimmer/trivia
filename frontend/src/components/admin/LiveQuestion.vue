<template>
  <div class="card stack">
    <div class="row between">
      <h2 style="margin: 0;">
        Now playing
        <span v-if="questionIndex && totalQuestions" class="muted" style="font-weight: 600; font-size: .9rem;">
          · Question {{ questionIndex }} / {{ totalQuestions }}
        </span>
      </h2>
      <span class="timer tag tag--pink" v-if="questionState === 'active'">{{ remaining }}s</span>
      <span class="tag tag--mint" v-else-if="questionState === 'revealed'">Revealed</span>
    </div>

    <div v-if="question" class="stack">
      <!-- Imported (poll) questions carry no photo and no author. -->
      <div v-if="question.photoImageId" class="photo-frame">
        <img :src="imageUrl(question.photoImageId, 'medium')" alt="" loading="lazy" decoding="async" />
        <div v-if="authorName" class="q-author">by {{ authorName }}</div>
      </div>
      <div class="q-card__text">{{ question.text }}</div>

      <div v-if="question.answerType === 'choice'" class="stack">
        <div
          v-for="(o, i) in question.options"
          :key="i"
          :class="['option-btn', i === Number(question.correct) && 'correct']"
        >
          <span class="option-btn__bullet">{{ letters[i] }}</span>{{ o }}
        </div>
      </div>
      <div v-else class="card card--mint center" style="padding: 14px;">
        <span class="muted bold" style="font-size: .78rem; letter-spacing: .12em; text-transform: uppercase;">Correct</span>
        <div style="font-family: var(--font-display); font-style: italic; font-weight: 900; font-size: 1.6rem; margin-top: 4px;">{{ question.correct }}</div>
      </div>
    </div>
    <div v-else class="muted center">No active question.</div>

    <div class="row wrap">
      <button class="btn-primary btn-lg" v-if="!question" @click="emit('activate-next')">▶ Start first question</button>
      <button class="btn-warn btn-lg" v-if="questionState === 'active'" @click="emit('reveal')">Reveal answer</button>
      <button class="btn-primary btn-lg" v-if="questionState === 'revealed'" @click="emit('next')">
        {{ isLastQuestion ? '🏆 Reveal winner' : 'Next question →' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { imageUrl } from '@/services/images'
import type { Question } from '@/types'

const props = defineProps<{
  question: Question | null
  questionState: string | undefined
  remaining: number
  authorName: string
  questionIndex?: number
  totalQuestions?: number
}>()

const isLastQuestion = computed(() =>
  !!props.questionIndex && !!props.totalQuestions && props.questionIndex >= props.totalQuestions
)

const emit = defineEmits<{
  (e: 'activate-next'): void
  (e: 'reveal'): void
  (e: 'next'): void
}>()

const letters = ['A', 'B', 'C', 'D']
</script>
