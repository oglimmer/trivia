# Trivia

Mobile-first, real-time trivia game. Players upload a photo and a question; the host runs the round; everyone answers live; scores are revealed with a podium finish.

Stack: **Vue 3 + Vite** (frontend), **Go** (backend, chi + gorilla/websocket + pgx), **Postgres**. Single Docker Compose for the whole thing. Live updates via WebSocket — no page refreshes.

## Quick start

```bash
cp .env.example .env             # edit ADMIN_PASSWORD, optionally set ANTHROPIC_API_KEY
docker compose up -d
```

- Players: <http://localhost:5173/>
- Admin: <http://localhost:5173/admin> (password from `.env`)

The backend auto-runs `migrations/*.sql` on boot. Re-running them is safe (everything is `IF NOT EXISTS`).

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

A **game** is created by the admin with a short code (e.g. `abc1`). It moves through three states:

1. **setup** — players join with name + photo, then submit one question each (photo + text + answers). Players can revise their question until the host moves on.
2. **game** — the host runs through every question in randomized order. Per question:
   - host **activates** it → players see it and submit answers (≤30s window for full time bonus)
   - host **reveals** → everyone sees the correct answer and a per-round result card
   - host clicks **next** → repeat, or finish if no questions remain
3. **finished** — players see a 3rd → 2nd → 1st spotlight, then the full ladder (their own row highlighted).

### Scoring

Two components, added together when an answer is correct:

```
points = base + timeBonus
```

| Question type        | Base | Notes                                    |
| -------------------- | ---- | ---------------------------------------- |
| yes/no               | 100  |                                          |
| 2-option choice      | 100  |                                          |
| 3-option choice      | 200  |                                          |
| 4-option choice      | 300  |                                          |
| number guess         | 300  | full credit within 0.5% (or ±1) of target |

```
timeBonus = base × 0.5 × max(0, 1 − responseMs / 30000)
```

…so the fastest answer at t=0 earns up to base × 1.5; at t=30s the bonus is zero. Wrong answers get 0 points except for **number guesses**, which award partial credit (up to base × 0.6) when within 25% of the target.

See `backend/internal/game/scoring.go` and `scoring_test.go`.

## Project layout

```
trivia/
├── docker-compose.yml
├── .env.example
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── api/        # HTTP + WS handlers, game flow, broadcasts
│   │   ├── auth/       # JWT for admin
│   │   ├── ai/         # Claude proxy for the suggest button
│   │   ├── db/         # pgx pool + queries + migrations runner
│   │   ├── game/       # scoring (pure functions, unit tested)
│   │   └── ws/         # generic websocket hub
│   └── migrations/0001_init.sql
└── frontend/
    ├── nginx.conf
    └── src/
        ├── pages/      # Landing, Join, Setup, Game, Results, Admin*
        ├── components/ # PhotoPicker, Spotlight
        ├── services/   # api.js, ws.js
        └── stores/game.js
```

## Development

Run the backend tests:

```bash
cd backend && go test ./...
```

Build the frontend without Docker:

```bash
cd frontend && npm install && npm run build
```

Hot-reload dev (talks to backend in Docker on `:8080`):

```bash
cd frontend && npm run dev
```

## API reference (short)

Public:
- `GET  /api/games/{code}` — basic game info for the join screen
- `POST /api/games/{code}/join` — `{name, photoB64}` → `{token, userId, gameId}`
- `GET  /api/me` — `X-Player-Token` header
- `PUT  /api/games/{code}/questions` — submit/update the player's question
- `GET  /api/games/{code}/leaderboard`
- `POST /api/ai/suggest` — `{hint, answerType, photoB64}` → `{text, options, correct}`

Admin (`Authorization: Bearer <jwt>`):
- `POST /api/admin/login` — `{password}` → `{token}`
- `GET  /api/admin/games`, `POST /api/admin/games`
- `GET  /api/admin/games/{code}`
- `POST /api/admin/games/{code}/state` — `{state: "setup"|"game"|"finished"}`
- `POST /api/admin/games/{code}/activate` — optional `{questionId}` (else next by sort order)
- `POST /api/admin/games/{code}/reveal`
- `POST /api/admin/games/{code}/next`
- `POST /api/admin/games/{code}/finish`

WebSocket at `/ws`:
- Player: `?token=<playerToken>`
- Admin:  `?role=admin&token=<adminJWT>&code=<gameCode>`

Inbound (from client):
- `{type:"answer", data:{questionId, value}}` — `value` shape depends on answer type (`"yes"|"no"`, integer index, or number).

Outbound (from server):
- `gameState` — the current view; admins additionally get `correct` and `questionsAdmin`.
- `users` — list of joined players (no tokens).
- `playerAnswered` — admin only; fires as each player submits.
- `answerAck` — to the submitting player, with `responseMs`.

## Gaps to plan & next steps

The plan in `plan.md` is fully implemented. The items below are either deliberate trade-offs or polish that didn't fit the initial build.

### Deliberate trade-offs

- **Photos stored as base64 in Postgres** (rather than an object store). Per your choice — simple and works offline, but the DB grows quickly with high-resolution photos. The frontend resizes to 1024px / JPEG q=0.82 before upload to keep this manageable (~100–500 KB per photo).
- **Single admin via env var**, not a `users` table. Matches the plan's "an admin has to authenticate" without overbuilding.
- **Randomized question order is set once on entering game mode**, not re-shuffled mid-game.
- **Players see only their own answer ack** during a round, not who else has answered (admin sees the live tally). Avoids social pressure / spoilers.
- **All answers shown on reveal**, not just the correct ones. The plan says "show the people who were right" — showing everyone with ✓/✗ is more transparent and lets losers see how close they were.

### Real gaps & nice-to-haves

- **No HTTPS / WSS**. Fine for LAN play; behind a reverse proxy in prod.
- **No rate limiting** on `/api/games/{code}/join` or `/api/ai/suggest` — both are spammable. Add a per-IP token bucket if exposing publicly.
- **JWT secret defaults to a dev value** if `JWT_SECRET` is unset. Loud-fail on startup would be safer.
- **No tests for the WebSocket layer** beyond a manual smoke script (`/tmp/ws_smoke.mjs`). Worth adding an integration test that boots a real `httptest.Server` + pgx with `testcontainers`.
- **Migrations are idempotent `*.sql` files**, not versioned with a tool like `goose`/`atlas`. Adding a column today is fine; renaming one needs a tool.
- **No game cleanup**. Old games and their base64 photos linger forever. Easy win: a daily job that deletes games older than N days.
- **The 30-second answer window and time-bonus shape are hardcoded** in `scoring.go`. Promoting them to per-game settings would let hosts tune difficulty.
- **No "rewind"**: admin can only move forward through questions, can't re-open a revealed one.
- **No host-side controls to skip / reorder questions** before the game starts. Today the order is random on transition to `game`.
- **No per-player "edit my name/photo" UI** after joining (the `PUT /api/me` endpoint exists; just unwired).
- **PWA basics missing**: no manifest, no service worker, no offline cache. App still works fine in a normal mobile browser.
- **AI suggestion**: returns the model's first JSON block; on rare malformed output the user gets the raw error. Could retry or wrap in a structured-output flow.
- **Accessibility**: the option buttons have keyboard focus and decent contrast, but there's no formal WCAG pass — screen-reader labels on the iconic buttons (camera, library, ✓, ✕) would help.
- **The podium reveal in `Results.vue` is one route with phases** rather than three separate URLs. Functionally identical to "different pages" but the back button skips the whole sequence.

### If I were to keep building

1. Add a real integration test (postgres + WS + scoring + reveal) using `testcontainers-go`.
2. Move photos to a `BYTEA` column or S3/MinIO; serve them via a signed-URL endpoint.
3. Per-game settings panel: answer window, point shape, reveal autoplay.
4. Multi-admin / SSO if this ever needs to scale past one host.
5. PWA install + offline page so players don't lose state if their connection blips.
