<template>
  <main class="stack-lg" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__sparkle s3" aria-hidden="true">★</span>

      <span class="hero__eyebrow">{{ t('heroEyebrow') }}</span>
      <h1 class="hero__title">{{ t('heroMottoPrefix') }}<em>{{ t('heroMottoEm') }}</em>,<br>{{ t('heroMottoSuffix') }}</h1>
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

    <!--
      The two formats, each shown as the thing it actually is: classic scores one
      right answer, Company Consensus scores the whole spread. The sample rows
      below are illustrations, not a live game.
    -->
    <section class="formats">
      <h2 class="formats__heading">{{ t('formatsHeading') }}</h2>
      <p class="formats__lede">{{ t('formatsLede') }}</p>

      <div class="formats__grid">
        <article class="fmt fmt--classic">
          <h3 class="fmt__name">{{ t('classicName') }}</h3>
          <p class="fmt__who">{{ t('classicWho') }}</p>

          <figure class="fmt__demo">
            <figcaption class="fmt__caption">{{ t('classicCaption') }}</figcaption>
            <p class="fmt__q">{{ t('classicQuestion') }}</p>
            <ul class="fmt__rows">
              <li
                v-for="opt in classicOptions"
                :key="opt.key"
                :class="['fmt__row', opt.correct && 'fmt__row--won']"
              >
                <span class="fmt__mark" aria-hidden="true">{{ opt.correct ? '✓' : '·' }}</span>
                <span class="fmt__label">{{ t(opt.key) }}</span>
                <span class="fmt__pts">{{ opt.correct ? '+142' : '0' }}</span>
              </li>
            </ul>
          </figure>

          <p class="fmt__rule">{{ t('classicRule') }}</p>
        </article>

        <article ref="pollCard" :class="['fmt', 'fmt--poll', revealed && 'is-revealed']">
          <h3 class="fmt__name">{{ t('pollName') }}</h3>
          <p class="fmt__who">{{ t('pollWho') }}</p>

          <figure class="fmt__demo">
            <figcaption class="fmt__caption">{{ t('pollCaption') }}</figcaption>
            <p class="fmt__q">{{ t('pollQuestion') }}</p>
            <ol class="fmt__rows">
              <li
                v-for="(opt, i) in pollOptions"
                :key="opt.key"
                class="fmt__row"
                :style="{
                  '--delay': `${i * 90}ms`,
                  '--w': `${Math.round((opt.count / pollTop) * 100)}%`,
                }"
              >
                <span class="fmt__rank">{{ i + 1 }}</span>
                <span class="fmt__cell">
                  <span class="fmt__label">{{ t(opt.key) }}</span>
                  <span class="fmt__bar" aria-hidden="true"><i class="fmt__fill"></i></span>
                </span>
                <span class="fmt__pts">{{ opt.count }}</span>
              </li>
            </ol>
          </figure>

          <p class="fmt__rule">{{ t('pollRule') }}</p>
        </article>
      </div>
    </section>

    <div class="host">
      <span class="host__ask">{{ t('hosting') }}</span>
      <RouterLink to="/admin" class="btn-ghost btn-sm">{{ t('openAdmin') }}</RouterLink>
      <p class="host__note">{{ t('hostBoardNote') }}</p>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
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

// Sample rows for the two format tiles. Illustrative only — the labels are
// translated like any other copy, the numbers are not.
const classicOptions = [
  { key: 'classicOpt1', correct: false },
  { key: 'classicOpt2', correct: true },
  { key: 'classicOpt3', correct: false },
  { key: 'classicOpt4', correct: false },
]

const pollOptions = [
  { key: 'pollOpt1', count: 41 },
  { key: 'pollOpt2', count: 27 },
  { key: 'pollOpt3', count: 12 },
  { key: 'pollOpt4', count: 9 },
  { key: 'pollOpt5', count: 5 },
]
const pollTop = pollOptions[0].count

// The consensus bars grow once, when the tile scrolls into view — the same
// beat the projector board has when a question closes. Reduced motion is
// handled globally in styles.css, which kills the transition but not the class.
const pollCard = ref<HTMLElement | null>(null)
const revealed = ref(false)
let observer: IntersectionObserver | null = null

onMounted(async () => {
  codeInput.value?.focus()

  if (pollCard.value && typeof IntersectionObserver !== 'undefined') {
    observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          revealed.value = true
          observer?.disconnect()
          observer = null
        }
      },
      { threshold: 0.2 },
    )
    observer.observe(pollCard.value)
  } else {
    revealed.value = true
  }

  await store.loadMe()
  if (store.me && store.game) {
    routeForState(store.game.code, store.game.state)
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
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
    err.value = errMsg(e, t('errorGameNotFound'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ---------- The two formats ---------- */
.formats__heading {
  text-align: center;
  margin: 0 0 6px;
}
.formats__lede {
  text-align: center;
  margin: 0 0 18px;
  color: var(--muted);
  font-size: .92rem;
}

.formats__grid {
  display: grid;
  gap: 16px;
}
@media (min-width: 700px) {
  .formats__grid { grid-template-columns: 1fr 1fr; }
}

.fmt {
  --accent: var(--pink);
  --accent-soft: var(--pink-2);
  position: relative;
  display: flex;
  flex-direction: column;
  background: var(--paper);
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-2);
  padding: 30px 18px 18px;
  overflow: hidden;
}
.fmt--poll {
  --accent: var(--blue);
  --accent-soft: var(--blue-2);
}
/* One band per format — the same accent the rows below are scored in. */
.fmt::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 12px;
  background: var(--accent);
  border-bottom: var(--bw) solid var(--ink);
}

.fmt__name {
  margin: 0 0 4px;
  font-size: clamp(1.3rem, 4.5vw, 1.5rem);
}
.fmt__who {
  margin: 0 0 14px;
  font-size: .92rem;
  line-height: 1.45;
  color: var(--ink-soft);
}

/* The miniature of the format's own screen. */
/* Grows so the shorter tile keeps its slack inside the frame — a four-row
   screen with room at the bottom, not a hole between the frame and the rule. */
.fmt__demo {
  flex: 1;
  margin: 0 0 14px;
  padding: 12px;
  background: var(--cream);
  border: var(--bw) dashed var(--ink);
  border-radius: var(--r);
}
.fmt__caption {
  font-size: .68rem;
  font-weight: 800;
  letter-spacing: .12em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 8px;
}
.fmt__q {
  margin: 0 0 10px;
  font-weight: 800;
  font-size: .95rem;
  line-height: 1.3;
  color: var(--ink);
}

.fmt__rows {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fmt__row {
  display: grid;
  grid-template-columns: 1.1rem 1fr auto;
  align-items: center;
  gap: 8px;
  padding: 7px 9px;
  background: var(--paper);
  border: 2px solid var(--ink);
  border-radius: var(--r-sm);
  font-size: .85rem;
}

/* Classic: one row wins, the rest are flat zeroes. */
.fmt__mark {
  font-weight: 900;
  color: var(--muted);
  text-align: center;
}
.fmt__row--won {
  background: var(--accent-soft);
}
.fmt__row--won .fmt__mark { color: var(--accent); }
.fmt__row--won .fmt__label { font-weight: 800; }

/* Consensus: every row carries weight, so every row carries a bar. The bar sits
   under its own label rather than behind it — a fill edge crossing the words
   was unreadable. */
.fmt__rank {
  font-family: var(--font-mono);
  font-size: .75rem;
  font-weight: 700;
  color: var(--muted);
  text-align: center;
}
.fmt__cell {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
}
.fmt__bar {
  display: block;
  height: 7px;
  background: var(--cream);
  border: 2px solid var(--ink);
  border-radius: 999px;
  overflow: hidden;
}
.fmt__fill {
  display: block;
  height: 100%;
  width: 0;
  background: var(--accent);
  transition: width .55s cubic-bezier(.22, 1, .36, 1);
  transition-delay: var(--delay, 0ms);
}
.is-revealed .fmt__fill { width: var(--w, 0%); }

.fmt__label {
  min-width: 0;
  overflow-wrap: anywhere;
}
.fmt__pts {
  font-family: var(--font-mono);
  font-size: .8rem;
  font-weight: 700;
  color: var(--ink);
}

/* `auto` keeps the two rules on the same baseline when one tile has fewer rows;
   the gap above comes from the demo's own margin so the rule never touches it. */
.fmt__rule {
  margin: auto 0 0;
  padding-top: 12px;
  border-top: 2px solid var(--ink);
  font-size: .85rem;
  font-weight: 700;
  line-height: 1.4;
  color: var(--ink);
}

/* ---------- Host row ---------- */
.host {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 12px;
  text-align: center;
}
.host__ask {
  font-size: .9rem;
  font-weight: 700;
  color: var(--muted);
}
.host__note {
  flex-basis: 100%;
  margin: 0;
  font-size: .82rem;
  color: var(--muted);
}
</style>
