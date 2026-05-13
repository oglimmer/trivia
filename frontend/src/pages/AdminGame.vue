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
              <img class="avatar" :src="q.photoB64" alt="" />
              <div style="flex: 1; min-width: 0;">
                <div class="bold">{{ q.text }}</div>
                <div class="muted" style="font-size: .85rem;">
                  <span class="kbd" style="padding: 1px 6px; font-size: .75rem;">{{ q.answerType }}</span>
                  · by {{ userName(q.userId) }}
                </div>
              </div>
              <button class="btn-danger btn-sm" :disabled="deletingQuestion === q.id" @click="removeQuestion(q)">
                {{ deletingQuestion === q.id ? '…' : 'Delete' }}
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
            <button class="btn-danger btn-sm" :disabled="deletingUser === u.id" @click="removeUser(u)" style="margin-left: auto;">
              {{ deletingUser === u.id ? '…' : 'Remove' }}
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
          <span class="timer tag tag--pink" v-if="game?.questionState === 'active'">{{ elapsed }}s</span>
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
    </template>

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

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../services/api.js'
import { onMessage, wsConnectAdmin, disconnect } from '../services/ws.js'
import { useGameStore } from '../stores/game.js'
import { confirm } from '../services/dialog.js'

const props = defineProps({ code: String })
const router = useRouter()
const store = useGameStore()

const game = ref(null)
const users = ref([])
const questions = ref([])
const currentQ = ref(null)
const leaderboard = ref([])
const playerAnswered = ref(new Set())
const online = ref(new Set())
const err = ref('')
const elapsed = ref(0)
const deletingUser = ref('')
const deletingQuestion = ref('')
let tick = null
let stopListening = null
const letters = ['A', 'B', 'C', 'D']

const answeredUsers = computed(() => users.value.filter(u => playerAnswered.value.has(u.id)))
const onlineCount = computed(() => users.value.filter(u => online.value.has(u.id)).length)

function userName(id) {
  const u = users.value.find(u => u.id === id)
  return u ? u.name : '...'
}
function hasQuestion(uid) {
  return questions.value.some(q => q.userId === uid)
}

async function load() {
  try {
    const r = await api.adminGame(props.code)
    game.value = r.game
    users.value = r.users || []
    questions.value = r.questions || []
    online.value = new Set(r.online || [])
  } catch (e) {
    if (String(e.message).toLowerCase().includes('unauthorized')) {
      store.logoutAdmin(); router.replace('/admin'); return
    }
    err.value = e.message
  }
}

function applyState(d) {
  game.value = {
    ...game.value,
    code: d.code, name: d.name, state: d.state,
    questionState: d.questionState,
    currentQuestionId: d.currentQuestionId,
    questionStartedAt: d.questionStartedAt,
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
    if (m.type === 'gameState') applyState(m.data)
    else if (m.type === 'users') users.value = m.data || []
    else if (m.type === 'questionsAdmin') questions.value = m.data || []
    else if (m.type === 'playerAnswered') playerAnswered.value.add(m.data.userId)
    else if (m.type === 'presence') online.value = new Set(m.data.online || [])
  })
  wsConnectAdmin(localStorage.getItem('adminToken'), props.code)

  tick = setInterval(() => {
    if (game.value?.questionState === 'active' && game.value?.questionStartedAt) {
      const s = new Date(game.value.questionStartedAt).getTime()
      elapsed.value = Math.floor((Date.now() - s) / 1000)
    }
  }, 250)
})

onUnmounted(() => {
  if (tick) clearInterval(tick)
  if (stopListening) stopListening()
  disconnect()
})

watch(() => game.value && game.value.state, () => { /* stay on page */ })

async function startGame() {
  err.value = ''
  try { await api.adminSetState(props.code, 'game') }
  catch (e) { err.value = e.message }
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
  try { await api.adminFinish(props.code) } catch (e) { err.value = e.message }
}

async function activateNext() {
  try { await api.adminActivate(props.code, null) } catch (e) { err.value = e.message }
}

async function reveal() {
  try { await api.adminReveal(props.code) } catch (e) { err.value = e.message }
}

async function next() {
  try {
    const r = await api.adminNext(props.code)
    if (r && r.done) { /* state arrives via ws */ }
  } catch (e) { err.value = e.message }
}

async function removeUser(u) {
  const ok = await confirm({
    title: `Remove ${u.name}?`,
    message: 'Their submission and any answers will also be deleted. This cannot be undone.',
    confirmLabel: 'Remove',
    cancelLabel: 'Keep',
    tone: 'danger',
    icon: '🗑',
  })
  if (!ok) return
  err.value = ''
  deletingUser.value = u.id
  try {
    await api.adminDeleteUser(props.code, u.id)
  } catch (e) {
    err.value = e.message
  } finally {
    deletingUser.value = ''
  }
}

async function removeQuestion(q) {
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
    await api.adminDeleteQuestion(props.code, q.id)
  } catch (e) {
    err.value = e.message
  } finally {
    deletingQuestion.value = ''
  }
}
</script>

<style scoped>
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
</style>
