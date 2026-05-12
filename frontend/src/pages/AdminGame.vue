<template>
  <main class="stack">
    <div class="card stack">
      <div class="row between">
        <div>
          <h1 style="margin: 0;">{{ code }}</h1>
          <div class="muted">{{ game?.name || '(no name)' }}</div>
        </div>
        <span class="tag">{{ game?.state || '…' }}</span>
      </div>
      <div class="row" style="flex-wrap: wrap; gap: 8px;">
        <button v-if="game?.state === 'setup'" class="btn-primary" @click="startGame">▶ Start game</button>
        <button v-if="game?.state === 'game'" class="btn-danger" @click="endGame">⏹ End game</button>
        <RouterLink to="/admin/games" class="muted" style="margin-left: auto;">← All games</RouterLink>
      </div>
      <div v-if="err" class="error">{{ err }}</div>
    </div>

    <!-- SETUP MODE: show submitted questions -->
    <template v-if="game?.state === 'setup'">
      <div class="card">
        <div class="row between" style="margin-bottom: 8px;">
          <h2>Submitted questions</h2>
          <span class="tag">{{ questions.length }}</span>
        </div>
        <div class="stack">
          <div v-for="q in questions" :key="q.id" class="card" style="background: var(--surface-2);">
            <div class="row" style="gap: 12px;">
              <img class="avatar" :src="q.photoB64" alt="" />
              <div style="flex: 1; min-width: 0;">
                <div style="font-weight: 600;">{{ q.text }}</div>
                <div class="muted" style="font-size: .85rem;">
                  <code>{{ q.answerType }}</code> · by {{ userName(q.userId) }}
                </div>
              </div>
            </div>
            <details style="margin-top: 8px;">
              <summary class="muted">Show answer</summary>
              <div v-if="q.answerType === 'choice'">
                <div v-for="(o, i) in q.options" :key="i" :class="['option-btn', i === Number(q.correct) && 'correct']" style="margin-top: 6px;">
                  {{ o }}
                </div>
              </div>
              <div v-else>{{ q.correct }}</div>
            </details>
          </div>
          <div v-if="questions.length === 0" class="muted center">No submissions yet.</div>
        </div>
      </div>

      <div class="card">
        <div class="row between" style="margin-bottom: 8px;">
          <h2>Players</h2>
          <span class="tag">{{ users.length }}</span>
        </div>
        <ul class="ladder">
          <li v-for="u in users" :key="u.id">
            <img class="avatar" :src="u.photoB64" :alt="u.name" />
            <span>{{ u.name }}</span>
            <span class="pts">{{ hasQuestion(u.id) ? '✓' : '…' }}</span>
          </li>
        </ul>
      </div>
    </template>

    <!-- GAME MODE: control panel -->
    <template v-if="game?.state === 'game'">
      <div class="card stack">
        <div class="row between">
          <h2>Current question</h2>
          <span class="timer" v-if="game?.questionState === 'active'">{{ elapsed }}s</span>
        </div>
        <div v-if="currentQ" class="stack">
          <div class="photo-frame">
            <img :src="currentQ.photoB64" alt="" />
          </div>
          <div><strong>{{ currentQ.text }}</strong></div>
          <div class="muted">by {{ userName(currentQ.userId) }}</div>
          <div v-if="currentQ.answerType === 'choice'" class="stack">
            <div v-for="(o, i) in currentQ.options" :key="i" :class="['option-btn', i === Number(currentQ.correct) && 'correct']">{{ o }}</div>
          </div>
          <div v-else><span class="muted">Correct:</span> {{ currentQ.correct }}</div>
        </div>
        <div v-else class="muted">No active question.</div>

        <div class="row" style="flex-wrap: wrap;">
          <button class="btn-primary" v-if="!currentQ" @click="activateNext">▶ Start first question</button>
          <button class="btn-accent" v-if="game?.questionState === 'active'" @click="reveal">Reveal answer</button>
          <button v-if="game?.questionState === 'revealed'" class="btn-primary" @click="next">Next →</button>
        </div>
      </div>

      <div v-if="game?.questionState !== 'idle' && playerAnswered.size" class="card">
        <div class="row between" style="margin-bottom: 8px;">
          <h2>Live answers</h2>
          <span class="tag">{{ playerAnswered.size }}/{{ users.length }}</span>
        </div>
        <div class="row" style="flex-wrap: wrap; gap: 8px;">
          <div v-for="u in answeredUsers" :key="u.id" class="row" style="gap: 6px;">
            <img class="avatar" :src="u.photoB64" :alt="u.name" />
            <span>{{ u.name }}</span>
          </div>
        </div>
      </div>

      <div class="card">
        <h2>Leaderboard</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span>{{ s.userName }}</span>
            <span class="pts">{{ s.points }} pts</span>
          </li>
        </ol>
      </div>
    </template>

    <template v-if="game?.state === 'finished'">
      <div class="card">
        <h2>Final standings</h2>
        <ol class="ladder">
          <li v-for="(s, i) in leaderboard" :key="s.userId">
            <span class="rank">{{ i + 1 }}</span>
            <img class="avatar" :src="s.photoB64" :alt="s.userName" />
            <span>{{ s.userName }}</span>
            <span class="pts">{{ s.points }} pts</span>
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
const err = ref('')
const elapsed = ref(0)
let tick = null
let stopListening = null

const answeredUsers = computed(() => users.value.filter(u => playerAnswered.value.has(u.id)))

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
    playerAnswered.value = new Set() // reset for new round
  }
}

onMounted(async () => {
  if (!localStorage.getItem('adminToken')) { router.replace('/admin'); return }
  await load()

  stopListening = onMessage((m) => {
    if (m.type === 'gameState') applyState(m.data)
    else if (m.type === 'users') users.value = m.data
    else if (m.type === 'questionsAdmin') questions.value = m.data
    else if (m.type === 'playerAnswered') playerAnswered.value.add(m.data.userId)
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

watch(() => game.value && game.value.state, (s) => {
  if (s === 'finished') {
    // remain on page; leaderboard shown
  }
})

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
    if (r && r.done) {
      // game is now finished, state arrives via ws
    }
  } catch (e) { err.value = e.message }
}
</script>
