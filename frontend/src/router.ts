import { createRouter, createWebHistory } from 'vue-router'
import Landing from './pages/Landing.vue'
import Join from './pages/Join.vue'
import Setup from './pages/Setup.vue'
import Game from './pages/Game.vue'
import Results from './pages/Results.vue'
import Board from './pages/Board.vue'
import AdminLogin from './pages/AdminLogin.vue'
import AdminGames from './pages/AdminGames.vue'
import AdminGame from './pages/AdminGame.vue'
import AdminUsers from './pages/AdminUsers.vue'
import Impersonate from './pages/Impersonate.vue'
import Imprint from './pages/Imprint.vue'
import Privacy from './pages/Privacy.vue'
import Terms from './pages/Terms.vue'
import Developers from './pages/Developers.vue'
import ShowcaseHub from './pages/showcase/Hub.vue'
import ShowcaseAuth from './pages/showcase/Auth.vue'
import ShowcaseImages from './pages/showcase/Images.vue'
import ShowcaseDatabase from './pages/showcase/Database.vue'
import ShowcaseWebSocket from './pages/showcase/WebSocket.vue'
import ShowcaseScoring from './pages/showcase/Scoring.vue'
import ShowcaseAI from './pages/showcase/AI.vue'
import ShowcaseDeployment from './pages/showcase/Deployment.vue'
import NotFound from './pages/NotFound.vue'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(_to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    return { top: 0 }
  },
  routes: [
    { path: '/', component: Landing },
    { path: '/g/:code/join', component: Join, props: true },
    { path: '/g/:code/setup', component: Setup, props: true },
    { path: '/g/:code/play', component: Game, props: true },
    { path: '/g/:code/results', component: Results, props: true },
    // Projector / TV view. Read-only, no player token, not a participant.
    // `fullscreen` drops the app header/footer and the 760px column — this one
    // is sized for a room, not a hand.
    { path: '/g/:code/board', component: Board, props: true, meta: { fullscreen: true } },
    { path: '/admin', component: AdminLogin },
    { path: '/admin/games', component: AdminGames },
    { path: '/admin/users', component: AdminUsers },
    { path: '/admin/games/:code', component: AdminGame, props: true },
    { path: '/impersonate', component: Impersonate },
    { path: '/imprint', component: Imprint },
    { path: '/privacy', component: Privacy },
    { path: '/terms', component: Terms },
    { path: '/developers', component: Developers },
    { path: '/developers-showcase', component: ShowcaseHub },
    { path: '/developers-showcase/auth', component: ShowcaseAuth },
    { path: '/developers-showcase/images', component: ShowcaseImages },
    { path: '/developers-showcase/database', component: ShowcaseDatabase },
    { path: '/developers-showcase/websocket', component: ShowcaseWebSocket },
    { path: '/developers-showcase/scoring', component: ShowcaseScoring },
    { path: '/developers-showcase/ai', component: ShowcaseAI },
    { path: '/developers-showcase/deployment', component: ShowcaseDeployment },
    { path: '/:pathMatch(.*)*', component: NotFound },
  ],
})

export default router
