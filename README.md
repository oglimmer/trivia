# Trivia

Mobile-first, real-time trivia game. Players upload a photo and a question; the host runs the round; everyone answers live; scores are revealed with a podium finish.

Stack: **Vue 3 + Vite + TypeScript** (frontend), **Go** (backend, chi + gorilla/websocket + pgx), **Postgres 18**. Single Docker Compose for local dev; a Helm chart ships the same app to Kubernetes. Live updates via WebSocket — no page refreshes.

## Quick start

```bash
cp .env.example .env             # edit ADMIN_PASSWORD, optionally set ANTHROPIC_API_KEY
docker compose up -d
```

- Players: <http://localhost:5173/>
- Admin: <http://localhost:5173/admin> (password from `.env`)

The backend auto-runs `migrations/*.sql` on boot. Re-running them is safe (everything is `IF NOT EXISTS` / `IF NOT EXISTS ADD COLUMN`).

## Configuration

| Variable             | Default                     | Purpose                                          |
| -------------------- | --------------------------- | ------------------------------------------------ |
| `ADMIN_PASSWORD`     | `letmein`                   | Host login password.                             |
| `JWT_SECRET`         | dev value                   | Signs admin session tokens. Set to ≥32 random bytes in prod. |
| `ANTHROPIC_API_KEY`  | (empty)                     | Enables the **✨ Help me with AI** button.       |
| `ANTHROPIC_MODEL`    | `claude-sonnet-4-6`         | Model used for question suggestions.             |
| `POSTGRES_*`         | `trivia/trivia/trivia`      | DB credentials and host.                         |
| `BACKEND_PORT`       | `8080`                      | Host port for the API.                           |
| `FRONTEND_PORT`      | `5173`                      | Host port for the nginx-served Vue app.          |

The frontend proxies `/api` and `/ws` to the backend via nginx (prod) and Vite (dev), so the app is served from a single origin.

## How it works

A **game** is created by the admin with a short code (e.g. `abc1`) and an answer window in seconds (default 30, range 5–600). It moves through three states:

1. **setup** — players join with name + photo, then submit one question each (photo + text + answers). Players can revise their question and edit their own name/photo until the host moves on. The host can remove individual players or their questions in this state, and can change the answer window for the whole game.
2. **game** — the host runs through every question in randomized order. Per question:
   - host **activates** it → players see it and submit answers (within the game's answer window for full time bonus)
   - host **reveals** → everyone sees the correct answer and a per-round result card. The server also auto-reveals when the timer expires, so a distracted host can't strand players forever.
   - host clicks **next** → repeat, or finish if no questions remain
3. **finished** — players see a 3rd → 2nd → 1st spotlight, then the full ladder (their own row highlighted).

Auto-close timers survive a backend restart: on boot the server re-arms any in-flight question's deadline from `question_started_at + question_timeout_seconds`.

### Scoring

Yes/no and choice questions are scored at submit time. **Number** questions are scored at reveal — points depend on the whole field of guesses, not just yours — so the server re-ranks all answers when the question closes (manually or by timeout).

For yes/no and choice, the formula is `points = base + timeBonus` when correct:

| Question type        | Base | Notes                                    |
| -------------------- | ---- | ---------------------------------------- |
| yes/no               | 100  |                                          |
| 2-option choice      | 100  |                                          |
| 3-option choice      | 200  |                                          |
| 4-option choice      | 300  | +100 per extra option above 4            |
| number               | 300  | scored by rank — see below               |

```
timeBonus = base × 0.5 × max(0, 1 − responseMs / windowMs)
```

…where `windowMs` is the game's configured answer window (default 30 s). At `t=0` the fastest answer earns up to `base × 1.5`; at the window's end the bonus is zero.

**Number questions** are rank-scored: every guess is compared to the target and the three closest get points scaled by closeness. Exact-or-near guesses (within `max(|c|·0.5%, 1)` of the target) get the full base + time bonus. Non-exact top-3 finishers get partial credit with rank weights `[1.0, 0.66, 0.33]` and **no** time bonus; closeness is `max(0, 1 − diff / max(|c|·0.5, 10))`, so wild guesses earn little even if they make the top 3. Wrong answers outside the top 3, and all wrong answers on yes/no & choice, score 0.

See `backend/internal/game/scoring.go` and `scoring_test.go`.

## Project layout

```
trivia/
├── compose.yml
├── .env.example
├── oglimmer.sh                # build/push/release helper (Docker + kubectl)
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── api/        # HTTP + WS handlers, game flow, broadcasts, auto-close timers
│   │   ├── auth/       # JWT for admin
│   │   ├── ai/         # Claude proxy for the suggest button
│   │   ├── db/         # pgx pool + queries + migrations runner
│   │   ├── game/       # scoring (pure functions, unit tested)
│   │   └── ws/         # generic websocket hub
│   └── migrations/
│       ├── 0001_init.sql
│       └── 0002_question_timeout.sql
├── frontend/
│   ├── nginx.conf
│   └── src/
│       ├── pages/       # Landing, Join, Setup, Game, Results, Impersonate, Admin*
│       ├── components/  # AppHeader, PhotoPicker, Spotlight, Stepper, ConfirmDialog, ProfileDialog
│       ├── composables/ # errMsg, useModal, useQuestionCountdown
│       ├── services/    # api/ (admin, player, http), ws.ts, dialog.ts
│       └── stores/game.ts
└── helm/trivia/                # Helm chart (see Kubernetes section)
```

## Development

Run the backend tests:

```bash
cd backend && go test ./...
```

Build the frontend without Docker (type-check then bundle):

```bash
cd frontend && npm install && npm run build
```

Hot-reload dev (talks to backend in Docker on `:8080`):

```bash
cd frontend && npm run dev
```

The `oglimmer.sh` helper at the repo root wraps the common loops: `./oglimmer.sh start|stop|status|logs|test` for a local Go binary; `./oglimmer.sh build [-f|-b]` for Docker build + push + `kubectl rollout restart`; `./oglimmer.sh release --bump minor` to bump the frontend `package.json` and Helm `Chart.yaml` together, tag, and push.

## Kubernetes (Helm)

The `helm/trivia` chart deploys backend + frontend + (optional) bundled Postgres behind ingress-nginx with cert-manager-issued TLS. WebSockets ride the same origin over `/ws`; the chart's ingress annotations disable response buffering and bump the read/send timeouts so streams stay alive.

```bash
# 1. Seal the cluster secrets (one-time, against your kubeseal controller):
kubectl create secret generic trivia-secret \
  --namespace default --dry-run=client -o yaml \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -hex 16) \
  --from-literal=JWT_SECRET=$(openssl rand -hex 32) \
  --from-literal=ADMIN_PASSWORD=<choose> \
  --from-literal=ANTHROPIC_API_KEY=<sk-ant-...> \
  | kubeseal --format yaml > helm/trivia/templates/sealed-secret.yaml

# 2. Install
helm install trivia ./helm/trivia
```

Key `values.yaml` knobs: `publicBaseURL`, `anthropic.model`, `backend.image` / `frontend.image` (default `registry.oglimmer.com/trivia-{backend,frontend}:latest`), `postgres.enabled` (toggle off to point at `externalPostgres.host` instead), `ingress.hosts` / `ingress.tls`. The default cert-manager issuer is `oglimmer-com-dns` (DNS-01); change `ingress.annotations.cert-manager.io/cluster-issuer` and `ingress.tls[].secretName` for your own setup. See `helm/trivia/README.md` for more.

## API reference (short)

Public:
- `GET  /api/games/{code}` — basic game info for the join screen
- `POST /api/games/{code}/join` — `{name, photoB64}` → `{token, userId, gameId, code}`
- `GET  /api/me` — `X-Player-Token` header → `{user, game}`
- `PUT  /api/me` — `{name, photoB64}` (player edits own profile)
- `GET  /api/games/{code}/users` — list of joined players
- `GET  /api/games/{code}/questions` — questions for the game (`correct` only included once the game is `finished`)
- `PUT  /api/games/{code}/questions` — submit/update the player's question
- `GET  /api/games/{code}/leaderboard`
- `POST /api/ai/suggest` — `{hint, answerType, photoB64}` → `{text, options, correct}`

Admin (`Authorization: Bearer <jwt>`):
- `POST   /api/admin/login` — `{password}` → `{token}`
- `GET    /api/admin/games`, `POST /api/admin/games` (`{code?, name, questionTimeoutSeconds?}`)
- `GET    /api/admin/games/{code}`
- `DELETE /api/admin/games/{code}` — drops the game; connected clients get a `gameDeleted` frame
- `POST   /api/admin/games/{code}/state` — `{state: "setup"|"game"|"finished"}`
- `PUT    /api/admin/games/{code}/settings` — `{questionTimeoutSeconds}` (setup only)
- `POST   /api/admin/games/{code}/activate` — optional `{questionId}` (else next by sort order)
- `POST   /api/admin/games/{code}/reveal`
- `POST   /api/admin/games/{code}/next`
- `POST   /api/admin/games/{code}/finish`
- `DELETE /api/admin/games/{code}/users/{userId}` (setup only)
- `GET    /api/admin/games/{code}/users/{userId}/impersonate` — returns that player's token + a deep-link the host can hand them
- `DELETE /api/admin/games/{code}/questions/{questionId}` (setup only)

WebSocket at `/ws`:
- Player: `?token=<playerToken>`
- Admin:  `?role=admin&token=<adminJWT>&code=<gameCode>`

Inbound (from client):
- `{type:"answer", data:{questionId, value}}` — `value` shape depends on answer type (`"yes"|"no"`, integer index, or number).
- `{type:"ping"}` → `{type:"pong"}`.

Outbound (from server):
- `gameState` — the current view, including `questionTimeoutSeconds` and `serverNow` so the client can compute clock skew for an accurate countdown. Admins additionally get `correct` and `questionsAdmin`.
- `users` — list of joined players (no tokens).
- `presence` — admin only; `{online: [userId, ...]}` of players with at least one live connection. Sent on every player join/leave.
- `playerAnswered` — admin only; fires as each player submits.
- `answerAck` — to the submitting player, with `responseMs`.
- `gameDeleted` — sent right before the host deletes a game.

Connection lifecycle: server pings every 30 s with a 75 s read deadline. The client also closes the socket on `visibilitychange→hidden` / `pagehide` so the player drops from the admin's presence list within a network RTT — without this, a backgrounded mobile tab would appear online for ~75 s after the user left the app. On `visibilitychange→visible` / `pageshow` / `online`, the client reconnects.

## Gaps & next steps

### Deliberate trade-offs

- **Photos stored as base64 in Postgres** (rather than an object store). Simple and works offline, but the DB grows quickly with high-resolution photos. The frontend resizes to 1024 px / JPEG q=0.82 before upload to keep this manageable (~100–500 KB per photo).
- **Single admin via env var**, not a `users` table. Matches "an admin has to authenticate" without overbuilding.
- **Randomized question order is set once on entering game mode**, not re-shuffled mid-game.
- **Players see only their own answer ack** during a round, not who else has answered (admin sees the live tally). Avoids social pressure / spoilers.
- **All answers shown on reveal**, not just the correct ones — more transparent and lets losers see how close they were.
- **Number scoring is rank-based**, not threshold-based. A whole field of bad guesses still produces a podium; lone perfect guesses still win cleanly because of the exact-tolerance branch.

### Real gaps & nice-to-haves

- **No HTTPS / WSS in the compose setup**. Fine for LAN play; the Helm chart terminates TLS at ingress for prod.
- **No rate limiting** on `/api/games/{code}/join` or `/api/ai/suggest` — both are spammable. Add a per-IP token bucket if exposing publicly.
- **JWT secret defaults to a dev value** if `JWT_SECRET` is unset. Loud-fail on startup would be safer.
- **No tests for the WebSocket layer** beyond a manual smoke script. Worth adding an integration test that boots a real `httptest.Server` + pgx with `testcontainers`.
- **Migrations are idempotent `*.sql` files**, not versioned with a tool like `goose`/`atlas`. Adding a column today is fine; renaming one needs a tool.
- **No game cleanup**. Old games and their base64 photos linger forever. Easy win: a daily job that deletes games older than N days.
- **No host-side reorder**: the admin can delete a question in setup, but ordering is still randomized once on transition to `game`.
- **No "rewind"**: admin can only move forward through questions, can't re-open a revealed one.
- **PWA basics missing**: no manifest, no service worker, no offline cache. App still works fine in a normal mobile browser.
- **AI suggestion** returns the model's first JSON block; on rare malformed output the user gets the raw error. Could retry or wrap in a structured-output flow.
- **Accessibility**: the option buttons have keyboard focus and decent contrast, but there's no formal WCAG pass — screen-reader labels on the iconic buttons (camera, library, ✓, ✕) would help.
- **The podium reveal in `Results.vue` is one route with phases** rather than three separate URLs. Functionally identical to "different pages" but the back button skips the whole sequence.

### If I were to keep building

1. Add a real integration test (postgres + WS + scoring + reveal) using `testcontainers-go`.
2. Move photos to a `BYTEA` column or S3/MinIO; serve them via a signed-URL endpoint.
3. Per-game settings panel: point shape, reveal autoplay (the answer window is already configurable).
4. Multi-admin / SSO if this ever needs to scale past one host.
5. PWA install + offline page so players don't lose state if their connection blips.
