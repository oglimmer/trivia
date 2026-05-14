<template>
  <main class="stack-lg">
    <div class="card stack">
      <div class="row between" style="align-items: flex-start;">
        <div>
          <div class="muted bold" style="font-size: .78rem; letter-spacing: .14em; text-transform: uppercase;">Room code</div>
          <h1 class="mono" style="margin: 4px 0 6px; letter-spacing: .12em; text-transform: lowercase;">{{ code }}</h1>
          <div class="muted">{{ game?.name || '(untitled)' }}</div>
        </div>
        <span :class="['state-pill', `state-${game?.state || 'setup'}`]">{{ game?.state || '…' }}</span>
      </div>

      <div class="row wrap" style="gap: 10px;">
        <button v-if="game?.state === 'setup'" class="btn-primary btn-lg" @click="startGame">▶ Start game</button>
        <button v-if="game?.state === 'game'" class="btn-danger" @click="endGame">⏹ End game</button>
        <RouterLink to="/admin/games" class="btn-ghost btn-sm" style="margin-left: auto;">← All games</RouterLink>
      </div>

      <div v-if="game?.state === 'setup'" class="row wrap" style="gap: 10px; align-items: center;">
        <label for="timeout-input" class="bold" style="margin: 0;">Question timeout</label>
        <input
          id="timeout-input"
          v-model.number="timeoutDraft"
          type="number"
          min="5"
          max="600"
          step="1"
          style="width: 90px;"
        />
        <span class="muted">seconds</span>
        <button class="btn-ghost btn-sm" :disabled="savingTimeout || timeoutDraft === game?.questionTimeoutSeconds" @click="saveTimeout">
          {{ savingTimeout ? '…' : 'Save' }}
        </button>
      </div>
      <div v-if="game?.state === 'setup'" class="row wrap" style="gap: 10px; align-items: center;">
        <label for="scheduled-input" class="bold" style="margin: 0;">Scheduled start</label>
        <input
          id="scheduled-input"
          v-model="scheduledDraft"
          type="datetime-local"
        />
        <button class="btn-ghost btn-sm" :disabled="savingSchedule || !scheduledDirty" @click="saveSchedule">
          {{ savingSchedule ? '…' : 'Save' }}
        </button>
        <button v-if="scheduledDraft" class="btn-link btn-sm" :disabled="savingSchedule" @click="clearSchedule">
          Clear
        </button>
      </div>
      <div v-else-if="game" class="muted" style="font-size: .85rem;">
        Question timeout: <span class="bold">{{ game.questionTimeoutSeconds || 30 }}s</span>
        <span v-if="game.scheduledAt"> · Scheduled: <span class="bold">{{ formatScheduled(game.scheduledAt) }}</span></span>
      </div>

      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <!-- SETUP MODE -->
    <template v-if="game?.state === 'setup'">
      <div class="card card--cream">
        <div class="row between" style="margin-bottom: 12px;">
          <h2 style="margin: 0;">Submissions</h2>
          <span class="tag tag--yellow">{{ questions.length }}</span>
        </div>
        <div class="stack">
          <div v-for="q in questions" :key="q.id" class="card card--flat" style="background: var(--paper); border: 2px solid var(--ink); padding: 14px;">
            <div class="row" style="gap: 12px; align-items: flex-start;">
              <button
                type="button"
                class="avatar-btn"
                @click="previewImage = q.photoB64"
                :aria-label="`Preview photo by ${userName(q.userId)}`"
                title="Click to preview"
              >
                <img class="avatar" :src="q.photoB64" alt="" />
              </button>
              <div style="flex: 1; min-width: 0;">
                <div class="bold">{{ q.text }}</div>
                <div class="muted" style="font-size: .85rem;">
                  <span class="kbd" style="padding: 1px 6px; font-size: .75rem;">{{ q.answerType }}</span>
                  · by {{ userName(q.userId) }}
                </div>
              </div>
              <button class="btn-danger btn-sm btn-icon-sm" :disabled="deletingQuestion === q.id" @click="removeQuestion(q)" :aria-label="`Delete submission by ${userName(q.userId)}`" :title="`Delete submission`">
                <span v-if="deletingQuestion === q.id">…</span>
                <span v-else aria-hidden="true">🗑</span>
              </button>
            </div>
            <details style="margin-top: 10px;">
              <summary class="bold" style="cursor: pointer;">Reveal answer</summary>
              <div v-if="q.answerType === 'choice'" class="stack" style="margin-top: 8px;">
                <div
                  v-for="(o, i) in q.options"
                  :key="i"
                  :class="['option-btn', i === Number(q.correct) && 'correct']"
                >
                  <span class="option-btn__bullet">{{ letters[i] }}</span>{{ o }}
                </div>
              </div>
              <div v-else class="card card--mint" style="padding: 12px; margin-top: 8px; text-align: center;">
                <span class="bold" style="font-family: var(--font-display); font-style: italic; font-size: 1.4rem;">{{ q.correct }}</span>
              </div>
            </details>
          </div>
          <div v-if="questions.length === 0" class="muted center">No submissions yet.</div>
        </div>
      </div>

      <div class="card">
        <div class="row between" style="margin-bottom: 12px;">
          <h2 style="margin: 0;">Players</h2>
          <span class="tag tag--blue">{{ onlineCount }} / {{ users.length }} online</span>
        </div>
        <ul class="ladder">
          <li v-for="u in users" :key="u.id">
            <span class="avatar-wrap">
              <img class="avatar" :src="u.photoB64" :alt="u.name" />
              <span class="presence-dot" :class="{ 'presence-dot--on': online.has(u.id) }" :title="online.has(u.id) ? 'Online' : 'Offline'"></span>
            </span>
            <span class="bold" :class="{ 'muted': !online.has(u.id) }">{{ u.name }}</span>
            <span class="pts" :style="`color: ${hasQuestion(u.id) ? 'var(--mint)' : 'var(--muted)'};`">
              {{ hasQuestion(u.id) ? '✓ ready' : 'thinking…' }}
            </span>
            <button class="btn-ghost btn-sm btn-icon-sm" :disabled="copyingUser === u.id" @click="copyImpersonateLink(u)" style="margin-left: auto;" :aria-label="`Copy login link for ${u.name}`" :title="`Copy a link that signs you in as ${u.name}`">
              <span v-if="copyingUser === u.id">…</span>
              <span v-else-if="copiedUser === u.id" aria-hidden="true">✓</span>
              <span v-else aria-hidden="true">🔗</span>
            </button>
            <button class="btn-danger btn-sm btn-icon-sm" :disabled="deletingUser === u.id" @click="removeUser(u)" :aria-label="`Remove ${u.name}`" title="Remove player">
              <span v-if="deletingUser === u.id">…</span>
              <span v-else aria-hidden="true">🗑</span>
            </button>
          </li>
          <li v-if="!users.length" class="muted center" style="justify-content: center;">No players yet.</li>
        </ul>
      </div>
    </template>

    <!-- GAME MODE -->
    <template v-if="game?.state === 'game'">
      <div class="card stack">
        <div class="row between">
          <h2 style="margin: 0;">Now playing</h2>
          <span class="timer tag tag--pink" v-if="game?.questionState === 'active'">{{ remaining }}s</span>
          <span class="tag tag--mint" v-else-if="game?.questionState === 'revealed'">Revealed</span>
        </div>

        <div v-if="currentQ" class="stack">
          <div class="photo-frame">
            <img :src="currentQ.photoB64" alt="" />
            <div class="q-author">by {{ userName(currentQ.userId) }}</div>
          </div>
          <div class="q-card__text">{{ currentQ.text }}</div>

          <div v-if="currentQ.answerType === 'choice'" class="stack">
            <div
              v-for="(o, i) in currentQ.options"
              :key="i"
              :class="['option-btn', i === Number(currentQ.correct) && 'correct']"
            >
              <span class="option-btn__bullet">{{ letters[i] }}</span>{{ o }}
            </div>
          </div>
          <div v-else class="card card--mint center" style="padding: 14px;">
            <span class="muted bold" style="font-size: .78rem; letter-spacing: .12em; text-transform: uppercase;">Correct</span>
            <div style="font-family: var(--font-display); font-style: italic; font-weight: 900; font-size: 1.6rem; margin-top: 4px;">{{ currentQ.correct }}</div>
          </div>
        </div>
        <div v-else class="muted center">No active question.</div>

        <div class="row wrap">
          <button class="btn-primary btn-lg" v-if="!currentQ" @click="activateNext">▶ Start first question</button>
          <button class="btn-warn btn-lg" v-if="game?.questionState === 'active'" @click="reveal">Reveal answer</button>
          <button class="btn-primary btn-lg" v-if="game?.questionState === 'revealed'" @click="next">Next question →</button>
        </div>
      </div>

      <div v-if="game?.questionState !== 'idle' && playerAnswered.size" class="card card--cream">
        <div class="row between" style="margin-bottom: 12px;">
          <h2 style="margin: 0;">Live answers</h2>
          <span class="tag tag--mint">{{ playerAnswered.size }} / {{ users.length }}</span>
        </div>
        <div class="row wrap" style="gap: 10px;">
          <div v-for="u in answeredUsers" :key="u.id" class="who" style="box-shadow: 2px 2px 0 var(--ink);">
            <span class="avatar-wrap">
              <img class="avatar avatar-sm" :src="u.photoB64" :alt="u.name" />
              <span class="presence-dot presence-dot--sm" :class="{ 'presence-dot--on': online.has(u.id) }"></span>
            </span>
            <div class="who__meta"><span class="who__name">{{ u.name }}</span></div>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Leaderboard</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span class="bold">{{ s.userName }}</span>
            <span class="pts">{{ s.points }}</span>
          </li>
          <li v-if="!leaderboard.length" class="muted center" style="justify-content: center;">Scores will appear here.</li>
        </ol>
      </div>

      <div class="card">
        <div class="row between" style="margin-bottom: 12px;">
          <h2 style="margin: 0;">Players</h2>
          <span class="tag tag--blue">{{ onlineCount }} / {{ users.length }} online</span>
        </div>
        <div class="row wrap" style="gap: 8px;">
          <div
            v-for="u in playersByPresence"
            :key="u.id"
            class="player-chip"
            :class="{ 'player-chip--offline': !online.has(u.id), 'player-chip--answered': showAnsweredMarker && playerAnswered.has(u.id) }"
            :title="online.has(u.id) ? 'Online' : 'Offline'"
          >
            <span class="avatar-wrap">
              <img class="avatar avatar-sm" :src="u.photoB64" :alt="u.name" />
              <span class="presence-dot presence-dot--sm" :class="{ 'presence-dot--on': online.has(u.id) }"></span>
            </span>
            <span class="player-chip__name">{{ u.name }}</span>
            <span v-if="showAnsweredMarker && playerAnswered.has(u.id)" class="player-chip__check" aria-label="answered" title="Answered">✓</span>
          </div>
          <div v-if="!users.length" class="muted">No players.</div>
        </div>
      </div>
    </template>

    <transition name="dialog">
      <div
        v-if="previewImage"
        class="img-preview-backdrop"
        role="dialog"
        aria-modal="true"
        aria-label="Photo preview"
        @mousedown.self="previewImage = ''"
        @keydown.esc.prevent="previewImage = ''"
        tabindex="-1"
        ref="previewBackdrop"
      >
        <button
          type="button"
          class="img-preview-close"
          aria-label="Close preview"
          @click="previewImage = ''"
        >×</button>
        <img class="img-preview-img" :src="previewImage" alt="" />
      </div>
    </transition>

    <template v-if="game?.state === 'finished'">
      <div class="card stack">
        <h2>Final standings</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span class="bold">{{ s.userName }}</span>
            <span class="pts">{{ s.points }}</span>
          </li>
        </ol>
      </div>
    </template>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '@/services/api'
import { onMessage, wsConnectAdmin, disconnect } from '@/services/ws'
import { useGameStore } from '@/stores/game'
import { confirm } from '@/services/dialog'
import { useQuestionCountdown } from '@/composables/useQuestionCountdown'
import { useModalRef } from '@/composables/useModal'
import { errMsg } from '@/composables/errMsg'
import type { Game, GameStateMsg, LeaderboardEntry, Question, User } from '@/types'

const props = defineProps<{ code: string }>()
const router = useRouter()
const store = useGameStore()

const game = ref<Game | null>(null)
const users = ref<User[]>([])
const questions = ref<Question[]>([])
const currentQ = ref<Question | null>(null)
const leaderboard = ref<LeaderboardEntry[]>([])
const playerAnswered = ref<Set<string>>(new Set())
const online = ref<Set<string>>(new Set())
const err = ref('')
const timeoutDraft = ref(30)
const savingTimeout = ref(false)
const scheduledDraft = ref('')
const savingSchedule = ref(false)
const deletingUser = ref('')
const deletingQuestion = ref('')
const copyingUser = ref('')
const copiedUser = ref('')
const previewImage = ref('')
const previewBackdrop = ref<HTMLElement | null>(null)
const previewOpen = computed(() => !!previewImage.value)
useModalRef(previewOpen, () => previewBackdrop.value)
let stopListening: (() => void) | null = null
const letters = ['A', 'B', 'C', 'D']
// Server-anchored clock offset (ms). Refreshed on every gameState arrival.
const serverClockOffsetMs = ref(0)

const { remaining } = useQuestionCountdown(game, { serverClockOffsetMs, intervalMs: 250 })

const answeredUsers = computed(() => users.value.filter(u => playerAnswered.value.has(u.id)))
const onlineCount = computed(() => users.value.filter(u => online.value.has(u.id)).length)
const playersByPresence = computed(() => {
  const on = users.value.filter(u => online.value.has(u.id))
  const off = users.value.filter(u => !online.value.has(u.id))
  return [...on, ...off]
})
const showAnsweredMarker = computed(() => game.value?.questionState === 'active' || game.value?.questionState === 'revealed')

function userName(id: string): string {
  const u = users.value.find(u => u.id === id)
  return u ? u.name : '...'
}
function hasQuestion(uid: string): boolean {
  return questions.value.some(q => q.userId === uid)
}

async function load() {
  try {
    const r = await adminApi.getGame(props.code)
    game.value = r.game
    users.value = r.users || []
    questions.value = r.questions || []
    online.value = new Set(r.online || [])
    timeoutDraft.value = r.game?.questionTimeoutSeconds || 30
    scheduledDraft.value = isoToLocalInput(r.game?.scheduledAt)
  } catch (e) {
    const msg = errMsg(e)
    if (msg.toLowerCase().includes('unauthorized')) {
      store.logoutAdmin(); router.replace('/admin'); return
    }
    err.value = msg
  }
}

function applyState(d: GameStateMsg) {
  if (d.serverNow) {
    serverClockOffsetMs.value = new Date(d.serverNow).getTime() - Date.now()
  }
  game.value = {
    ...(game.value || {} as Game),
    code: d.code, name: d.name, state: d.state,
    questionState: d.questionState,
    currentQuestionId: d.currentQuestionId,
    questionStartedAt: d.questionStartedAt,
    questionTimeoutSeconds: d.questionTimeoutSeconds,
    scheduledAt: d.scheduledAt,
  }
  // Keep the edit field in sync when we're not actively editing.
  if (d.state === 'setup' && !savingTimeout.value) {
    timeoutDraft.value = d.questionTimeoutSeconds || 30
  }
  if (d.state === 'setup' && !savingSchedule.value) {
    scheduledDraft.value = isoToLocalInput(d.scheduledAt)
  }
  if (d.question) currentQ.value = d.question
  else if (d.questionState === 'idle') currentQ.value = null
  if (d.leaderboard) leaderboard.value = d.leaderboard
  if (d.questionState === 'active') {
    playerAnswered.value = new Set()
  }
}

onMounted(async () => {
  if (!localStorage.getItem('adminToken')) { router.replace('/admin'); return }
  await load()

  stopListening = onMessage((m) => {
    if (m.type === 'gameState') applyState(m.data as GameStateMsg)
    else if (m.type === 'users') users.value = (m.data as User[]) || []
    else if (m.type === 'questionsAdmin') questions.value = (m.data as Question[]) || []
    else if (m.type === 'playerAnswered') playerAnswered.value.add((m.data as { userId: string }).userId)
    else if (m.type === 'presence') online.value = new Set((m.data as { online?: string[] }).online || [])
  })
  wsConnectAdmin(localStorage.getItem('adminToken') || '', props.code)
})

onUnmounted(() => {
  if (stopListening) stopListening()
  disconnect()
})

watch(() => game.value && game.value.state, () => { /* stay on page */ })

async function startGame() {
  err.value = ''
  try { await adminApi.setState(props.code, 'game') }
  catch (e) { err.value = errMsg(e) }
}

async function saveTimeout() {
  err.value = ''
  savingTimeout.value = true
  try {
    await adminApi.updateSettings(props.code, { questionTimeoutSeconds: Number(timeoutDraft.value) || 30 })
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    savingTimeout.value = false
  }
}

// "datetime-local" inputs use the local TZ; converting to/from the ISO string
// the API speaks needs explicit Date hops so we don't ship UTC strings into
// the local-time field by mistake.
function isoToLocalInput(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
function localInputToIso(v: string): string | null {
  if (!v) return null
  const d = new Date(v)
  if (isNaN(d.getTime())) return null
  return d.toISOString()
}
function formatScheduled(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

const scheduledDirty = computed(() => {
  return scheduledDraft.value !== isoToLocalInput(game.value?.scheduledAt)
})

async function saveSchedule() {
  err.value = ''
  savingSchedule.value = true
  try {
    const iso = localInputToIso(scheduledDraft.value)
    await adminApi.updateSettings(props.code, { scheduledAt: iso })
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    savingSchedule.value = false
  }
}

async function clearSchedule() {
  err.value = ''
  savingSchedule.value = true
  try {
    await adminApi.updateSettings(props.code, { scheduledAt: null })
    scheduledDraft.value = ''
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    savingSchedule.value = false
  }
}

async function endGame() {
  const ok = await confirm({
    title: 'End the game now?',
    message: 'Players will see the final leaderboard. Remaining questions will be skipped.',
    confirmLabel: 'End game',
    cancelLabel: 'Keep playing',
    tone: 'danger',
    icon: '⏹',
  })
  if (!ok) return
  try { await adminApi.finish(props.code) } catch (e) { err.value = errMsg(e) }
}

async function activateNext() {
  try { await adminApi.activate(props.code, null) } catch (e) { err.value = errMsg(e) }
}

async function reveal() {
  try { await adminApi.reveal(props.code) } catch (e) { err.value = errMsg(e) }
}

async function next() {
  try {
    const r = await adminApi.next(props.code)
    if (r && r.done) { /* state arrives via ws */ }
  } catch (e) { err.value = errMsg(e) }
}

async function copyImpersonateLink(u: User) {
  err.value = ''
  copyingUser.value = u.id
  try {
    const r = await adminApi.impersonate(props.code, u.id)
    const url = `${window.location.origin}/impersonate#token=${encodeURIComponent(r.token)}`
    let ok = false
    if (navigator.clipboard && window.isSecureContext) {
      try { await navigator.clipboard.writeText(url); ok = true } catch {}
    }
    if (!ok) {
      const ta = document.createElement('textarea')
      ta.value = url
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      try { ok = document.execCommand('copy') } catch {}
      document.body.removeChild(ta)
    }
    if (!ok) {
      err.value = 'Could not copy. Link: ' + url
      return
    }
    copiedUser.value = u.id
    setTimeout(() => { if (copiedUser.value === u.id) copiedUser.value = '' }, 2000)
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    copyingUser.value = ''
  }
}

async function removeUser(u: User) {
  const ok = await confirm({
    title: `Remove ${u.name}?`,
    message: 'Their submission stays in the game — delete it separately if you want it gone.',
    confirmLabel: 'Remove',
    cancelLabel: 'Keep',
    tone: 'danger',
    icon: '🗑',
  })
  if (!ok) return
  err.value = ''
  deletingUser.value = u.id
  try {
    await adminApi.deleteUser(props.code, u.id)
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    deletingUser.value = ''
  }
}

async function removeQuestion(q: Question) {
  const ok = await confirm({
    title: 'Delete this submission?',
    message: `"${q.text}" — by ${userName(q.userId)}. The player stays in the game and can submit a new question.`,
    confirmLabel: 'Delete',
    cancelLabel: 'Keep',
    tone: 'danger',
    icon: '🗑',
  })
  if (!ok) return
  err.value = ''
  deletingQuestion.value = q.id
  try {
    await adminApi.deleteQuestion(props.code, q.id)
  } catch (e) {
    err.value = errMsg(e)
  } finally {
    deletingQuestion.value = ''
  }
}
</script>

<style scoped>
.btn-icon-sm {
  padding: 0;
  width: 36px;
  height: 36px;
  min-width: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  line-height: 1;
  flex-shrink: 0;
}

.state-pill {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 999px;
  border: 2px solid var(--ink);
  font-weight: 800;
  letter-spacing: .08em;
  text-transform: uppercase;
  font-size: .75rem;
  background: var(--paper);
  color: var(--ink);
  box-shadow: 2px 2px 0 var(--ink);
}
.state-pill.state-setup    { background: var(--blue-2); }
.state-pill.state-game     { background: var(--pink); color: var(--paper); }
.state-pill.state-finished { background: var(--mint-2); }

.avatar-wrap {
  position: relative;
  display: inline-flex;
}
.presence-dot {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #9ca3af;
  border: 2px solid var(--paper);
  box-sizing: border-box;
}
.presence-dot--on { background: #22c55e; }
.presence-dot--sm {
  width: 10px;
  height: 10px;
  right: -1px;
  bottom: -1px;
}

.player-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px 4px 4px;
  background: var(--paper);
  color: var(--ink);
  border: 2px solid var(--ink);
  border-radius: 999px;
  box-shadow: 2px 2px 0 var(--ink);
  font-weight: 800;
  font-size: .9rem;
  max-width: 220px;
  min-width: 0;
}
.player-chip__name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.player-chip--offline {
  opacity: .55;
  background: var(--cream-2, var(--paper));
}
.player-chip--offline .player-chip__name {
  text-decoration: line-through;
  text-decoration-thickness: 1px;
}
.player-chip--answered {
  background: var(--mint-2, var(--mint));
}
.player-chip__check {
  color: var(--mint);
  font-weight: 900;
  margin-left: 2px;
}
.player-chip--answered .player-chip__check {
  color: var(--ink);
}

.avatar-btn {
  padding: 0;
  border: 0;
  background: transparent;
  cursor: zoom-in;
  border-radius: 50%;
  flex-shrink: 0;
}
.avatar-btn:focus-visible {
  outline: 3px solid var(--blue);
  outline-offset: 2px;
}

.img-preview-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(26, 27, 38, .85);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 1100;
  cursor: zoom-out;
}
.img-preview-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r-lg);
  background: var(--paper);
  box-shadow: var(--shadow-3);
  cursor: default;
}
.img-preview-close {
  position: absolute;
  top: 16px;
  right: 16px;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: var(--bw) solid var(--ink);
  background: var(--paper);
  color: var(--ink);
  font-size: 1.5rem;
  font-weight: 800;
  line-height: 1;
  cursor: pointer;
  box-shadow: var(--shadow-1);
}
.img-preview-close:hover { background: var(--coral); color: var(--paper); }
</style>
