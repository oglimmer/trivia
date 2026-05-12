import { createRouter, createWebHistory } from 'vue-router'
import Landing from './pages/Landing.vue'
import Join from './pages/Join.vue'
import Setup from './pages/Setup.vue'
import Game from './pages/Game.vue'
import Results from './pages/Results.vue'
import AdminLogin from './pages/AdminLogin.vue'
import AdminGames from './pages/AdminGames.vue'
import AdminGame from './pages/AdminGame.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Landing },
    { path: '/g/:code/join', component: Join, props: true },
    { path: '/g/:code/setup', component: Setup, props: true },
    { path: '/g/:code/play', component: Game, props: true },
    { path: '/g/:code/results', component: Results, props: true },
    { path: '/admin', component: AdminLogin },
    { path: '/admin/games', component: AdminGames },
    { path: '/admin/games/:code', component: AdminGame, props: true },
  ],
})

export default router
