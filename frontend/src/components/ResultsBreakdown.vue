<template>
  <div class="stack-lg">
    <div v-if="loading" class="card card--cream center stack">
      <div class="spinner" aria-hidden="true"></div>
      <span class="muted">Loading breakdown…</span>
    </div>
    <div v-else-if="!questions.length" class="card card--cream center muted">
      No question data yet.
    </div>
    <p v-if="!loading && votable && questions.length" class="vote-hint">
      <template v-if="myVote">★ Thanks — your pick for best question is locked in.</template>
      <template v-else>Crown the best question — you get one vote, and it's final.</template>
    </p>
    <article
      v-for="(q, i) in questions"
      :key="q.questionId"
      :class="['card stack breakdown-card', { 'breakdown-card--winner': showCounts && isWinner(q) }]"
    >
      <header class="breakdown-card__head">
        <span class="breakdown-card__index">Q{{ i + 1 }}</span>
        <div class="breakdown-card__heading">
          <p class="breakdown-card__text">{{ q.text }}</p>
          <p v-if="q.authorName" class="breakdown-card__author muted">
            by {{ q.authorName }}
          </p>
        </div>
        <span
          v-if="showCounts"
          class="breakdown-card__votes"
          :class="{ 'breakdown-card__votes--winner': isWinner(q) }"
        >
          <span aria-hidden="true">{{ isWinner(q) ? '🏆' : '★' }}</span>
          {{ q.voteCount ?? 0 }}
          <span class="breakdown-card__votes-label">{{ (q.voteCount ?? 0) === 1 ? 'vote' : 'votes' }}</span>
        </span>
      </header>
      <div class="breakdown-card__body">
        <img
          v-if="q.photoImageId"
          class="breakdown-card__photo"
          :src="imageUrl(q.photoImageId, 'thumb')"
          :alt="q.text"
          loading="lazy"
          decoding="async"
        />
        <div class="breakdown-card__main">
          <div class="breakdown-summary">
            <span class="breakdown-summary__pill breakdown-summary__pill--correct">
              ✓ {{ q.correctCount }}
            </span>
            <span class="breakdown-summary__pill breakdown-summary__pill--wrong">
              ✗ {{ q.incorrectCount }}
            </span>
            <span
              v-if="q.noAnswerCount > 0"
              class="breakdown-summary__pill breakdown-summary__pill--skip"
            >
              — {{ q.noAnswerCount }}
            </span>
          </div>
          <ul class="breakdown-bars">
            <li
              v-for="(b, bi) in q.distribution"
              :key="bi"
              :class="['breakdown-bar', { 'breakdown-bar--correct': b.isCorrect }]"
            >
              <div class="breakdown-bar__row">
                <span class="breakdown-bar__label">
                  <span v-if="b.isCorrect" class="breakdown-bar__check" aria-label="correct">✓</span>
                  {{ b.label }}
                </span>
                <span class="breakdown-bar__count">
                  {{ b.count }} · {{ pct(b.count, denominator(q)) }}%
                </span>
              </div>
              <div class="breakdown-bar__track" aria-hidden="true">
                <div
                  class="breakdown-bar__fill"
                  :style="{ width: pct(b.count, denominator(q)) + '%' }"
                ></div>
              </div>
            </li>
          </ul>
        </div>
      </div>
      <footer v-if="votable" class="breakdown-card__vote">
        <button
          v-if="myVote === q.questionId"
          type="button"
          class="btn-primary btn-sm"
          disabled
        >★ Your pick</button>
        <button
          v-else-if="!myVote"
          type="button"
          class="btn-ghost btn-sm"
          :disabled="voting"
          @click="emit('vote', q.questionId)"
        >Vote best question</button>
      </footer>
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { imageUrl } from '@/services/images'
import type { QuestionResults } from '@/types'

const props = withDefaults(defineProps<{
  questions: QuestionResults[]
  loading?: boolean
  // When true, render the one-vote-per-player "best question" controls.
  votable?: boolean
  // Display the best-question vote tallies and winner highlight. Admin-only —
  // players must never see counts, or the running tally would bias their pick.
  showCounts?: boolean
  // The questionId this player has already voted for ('' = not voted yet).
  myVote?: string
  // A vote request is in flight — disables the buttons to prevent double-submit.
  voting?: boolean
}>(), { loading: false, votable: false, showCounts: false, myVote: '', voting: false })

const emit = defineEmits<{ (e: 'vote', questionId: string): void }>()

// The leading vote tally across all questions; cards matching it (and with at
// least one vote) get the winner treatment. A tie crowns every leader.
const topVotes = computed(() =>
  props.questions.reduce((max, q) => Math.max(max, q.voteCount ?? 0), 0),
)

function isWinner(q: QuestionResults): boolean {
  return topVotes.value > 0 && (q.voteCount ?? 0) === topVotes.value
}

function pct(count: number, total: number): number {
  if (!total || total <= 0) return 0
  return Math.round((count / total) * 100)
}

// Bars scale against the larger of answeredCount or totalPlayers so a question
// where most players skipped still shows a visibly empty track instead of
// inflating partial answers to 100%.
function denominator(q: QuestionResults): number {
  return Math.max(q.answeredCount, q.totalPlayers, 1)
}
</script>

<style scoped>
.breakdown-card {
  gap: 14px;
}
.breakdown-card__head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.breakdown-card__index {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  min-width: 36px;
  height: 36px;
  padding: 0 8px;
  border-radius: 18px;
  background: var(--cream-2);
  border: 2px solid var(--ink);
  font-family: var(--font-display);
  font-style: italic;
  font-weight: 900;
  font-size: .9rem;
}
.breakdown-card__heading {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.breakdown-card__text {
  margin: 6px 0 0;
  font-weight: 700;
  word-wrap: break-word;
}
.breakdown-card__author {
  margin: 0;
  font-size: .8rem;
}
.vote-hint {
  margin: 0;
  font-weight: 700;
  font-size: .9rem;
  color: var(--muted);
}
.breakdown-card--winner {
  border-color: var(--ink);
  box-shadow: 4px 4px 0 var(--ink);
  background: var(--mint-2);
}
.breakdown-card__votes {
  flex-shrink: 0;
  align-self: flex-start;
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  background: var(--paper);
  font-weight: 800;
  font-size: .85rem;
  white-space: nowrap;
}
.breakdown-card__votes--winner {
  background: var(--mint);
}
.breakdown-card__votes-label {
  font-weight: 600;
  font-size: .75rem;
  color: var(--muted);
}
.breakdown-card__vote {
  display: flex;
  justify-content: flex-end;
}
.breakdown-card__body {
  display: flex;
  gap: 14px;
  align-items: flex-start;
}
.breakdown-card__photo {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r);
  flex-shrink: 0;
}
.breakdown-card__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.breakdown-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  font-weight: 700;
  font-size: .85rem;
}
.breakdown-summary__pill {
  padding: 3px 10px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  background: var(--paper);
}
.breakdown-summary__pill--correct { background: var(--mint-2); }
.breakdown-summary__pill--wrong   { background: var(--pink-2); }
.breakdown-summary__pill--skip    { background: var(--cream-2); }

.breakdown-bars {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.breakdown-bar__row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  font-size: .9rem;
}
.breakdown-bar__label {
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.breakdown-bar__check {
  color: var(--ink);
}
.breakdown-bar__count {
  font-family: var(--font-display);
  font-style: italic;
  font-size: .85rem;
  color: var(--muted);
  flex-shrink: 0;
}
.breakdown-bar__track {
  margin-top: 4px;
  height: 12px;
  border: 2px solid var(--ink);
  border-radius: 999px;
  background: var(--paper);
  overflow: hidden;
}
.breakdown-bar__fill {
  height: 100%;
  background: var(--cream-2);
  transition: width .4s ease;
}
.breakdown-bar--correct .breakdown-bar__fill {
  background: var(--mint);
}
.breakdown-bar--correct .breakdown-bar__count {
  color: var(--ink);
}

@media (max-width: 480px) {
  .breakdown-card__body {
    flex-direction: column;
  }
  .breakdown-card__photo {
    width: 100%;
    height: 140px;
  }
}
</style>
