# Trivia

Mobile-first, real-time trivia game. Players upload a photo and a question; the host runs the round; everyone answers live; scores are revealed with a podium finish. A second **Company Consensus mode** flips the format: the host imports survey-derived questions and teams guess the *most popular* answer rather than the correct one (see "Company Consensus mode" below).

Stack: **Vue 3 + Vite + TypeScript** (frontend), **Go** (backend, chi + gorilla/websocket + pgx), **Postgres**. Single Docker Compose for local dev; a Helm chart ships the same app to Kubernetes. Live updates via WebSocket — no page refreshes. Optional SMTP integration sends players a one-click login link so they can rejoin a game on any device.

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
| `PUBLIC_BASE_URL`    | `http://localhost:5173`     | Origin used to build the magic-link URL in emails. Helm sets it from `publicBaseURL`. |
| `SMTP_ENABLED`       | `false`                     | When `false`, the backend just logs the magic-link URL instead of sending — dev/test flows still work without a relay. |
| `SMTP_HOST`          | (empty)                     | SMTP relay hostname.                             |
| `SMTP_PORT`          | `25`                        | SMTP relay port (typical: 25 plain, 587 STARTTLS, 465 implicit TLS). |
| `SMTP_TLS`           | `false`                     | Implicit TLS (SMTPS). Mutually exclusive with STARTTLS. |
| `SMTP_STARTTLS`      | `false`                     | Require STARTTLS upgrade. Fails closed if the server doesn't advertise it. With both `SMTP_TLS` and `SMTP_STARTTLS` false, no TLS is attempted (even if the server advertises STARTTLS) — use only for anonymous relays on a trusted network. |
| `SMTP_USER`          | (empty)                     | Optional PLAIN-auth username. Go's stdlib refuses to send credentials over a plaintext non-localhost connection, so pair this with TLS or STARTTLS. |
| `SMTP_PASSWORD`      | (empty)                     | Password to pair with `SMTP_USER`.               |
| `SMTP_FROM`          | `noreply@oglimmer.com`      | RFC 5322 `From:` address.                        |
| `SMTP_REPLY_EMAIL`   | `noreply@oglimmer.com`      | `Reply-To:` address.                             |
| `SMTP_REPLY_NAME`    | `Trivia-Helper`             | Display name for the reply address.              |
| `METRICS_TOKEN`      | (empty)                     | Bearer token required to scrape `/metrics`. Empty disables the endpoint (returns 404). |

The frontend proxies `/api` and `/ws` to the backend via nginx (prod) and Vite (dev), so the app is served from a single origin.

## How it works

A **game** is created by the admin with a short code (e.g. `abc1`), an answer window in seconds (default 30, range 5–600), and an optional **scheduled start** (date + time). The schedule is informational — the host still presses ▶ to start the game — but it changes the player UX (see "Scheduled start & magic-link login" below). It moves through three states:

1. **setup** — players join with name + photo, then submit one question each (photo + text + answers). Players can revise their question and edit their own name/photo until the host moves on. The host can remove individual players or their questions in this state, and can change the answer window or scheduled start for the whole game.
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
| poll (Company Consensus)   | —    | the chosen option's survey count — see below |

```
timeBonus = base × 0.5 × max(0, 1 − responseMs / windowMs)
```

…where `windowMs` is the game's configured answer window (default 30 s), passed into scoring from `games.question_timeout_seconds`. At `t=0` the fastest answer earns up to `base × 1.5`; at the window's end the bonus is zero. A 90 s question therefore decays over the full 90 s, not over a fixed 30 s.

**Number questions** are rank-scored: every guess is compared to the target and the three closest get points scaled by closeness. Exact-or-near guesses (within `max(|c|·0.5%, 1)` of the target) get the full base + time bonus. Non-exact top-3 finishers get partial credit with rank weights `[1.0, 0.66, 0.33]` and **no** time bonus; closeness is `max(0, 1 − diff / max(|c|·0.5, 10))`, so wild guesses earn little even if they make the top 3. Wrong answers outside the top 3, and all wrong answers on yes/no & choice, score 0.

See `backend/internal/game/scoring.go` and `scoring_test.go`.

## Company Consensus mode

A game created with `mode: "poll"` runs an entirely different format. Instead of
guessing the correct answer, teams guess **what most people said** in a survey
run before the event. Every one of the five options scores; the more people who
gave that answer, the more it is worth.

What changes:

| | classic | poll |
| --- | --- | --- |
| Who writes the questions | one per player, in setup | the host imports them |
| Photo per question | required | none |
| Answer options | 2–4, one correct | exactly 5, all scoring |
| Points | `base + timeBonus` by type | the option's survey count `+ timeBonus` |
| Question order on start | shuffled | as uploaded |
| Setup page for players | 3-step question editor | a lobby (teams just wait) |
| Leaderboard suspense tail | on | off (a live TV board is the point) |

**Teams** are modelled as ordinary players: one phone per team, the team name in
the name field. Nothing in the schema knows about rosters.

### Authoring the question set

The admin console has a normal editor for this: **add**, **edit**, **remove**,
and **↑ / ↓** to arrange the running order. Each question is a text field plus
five answer rows (answer + points). Validation is inline, so the save button
explains what is missing rather than failing on submit.

| Endpoint | |
| --- | --- |
| `POST /api/admin/games/{code}/questions` | add one |
| `PUT /api/admin/games/{code}/questions/{id}` | edit one, keeping its slot |
| `POST /api/admin/games/{code}/questions/{id}/move` | `{"direction":"up"\|"down"}` |
| `DELETE /api/admin/games/{code}/questions/{id}` | remove one |
| `POST /api/admin/games/{code}/questions/import` | replace the whole set from JSON |

All of them are **setup-only** and **poll-only**: once teams are playing, editing
the set would orphan their answers, and in a classic game the questions belong to
their player authors (`PUT` refuses any question with a non-NULL `user_id`).

The editor always shows a question's answers **ranked by points**, which is how
survey results arrive and how a human thinks about them. The stored order is
shuffled and never surfaced.

### Bulk import

Still available, collapsed under "Bulk import from JSON" in the editor —
convenient for dropping in a whole survey at once. It **replaces** every
host-authored question in the game in one transaction; everything stays editable
afterwards.

```json
{
  "questions": [
    {
      "text": "Name something you always forget to pack.",
      "answers": [
        {"text": "Toothbrush", "points": 41},
        {"text": "Phone charger", "points": 22},
        {"text": "Socks", "points": 11},
        {"text": "Sunscreen", "points": 7},
        {"text": "Passport", "points": 4}
      ]
    }
  ]
}
```

`points` is however many survey respondents gave that answer. Exactly 5 answers
per question, up to 50 questions, no duplicate answers within a question (case
insensitive). Player-written questions in the same game are left untouched.

Options are **shuffled on save** — on the single-question editor and the bulk
import alike, both of which go through `buildPollQuestion`. Without that the top
answer would always sit in the first slot and the game would collapse into
"always tap the top row".

Imported questions are stored with `user_id = NULL` — Postgres does not treat
NULLs as equal, so the `UNIQUE (game_id, user_id)` index tolerates a whole set of
them. The survey counts live on the `options` JSONB as `[{"text":…,"points":…}]`,
and `correct` holds the JSON literal `null` (the column is `NOT NULL`, and no
single answer is right).

**The point values are withheld from players and from the board until the host
reveals.** They are the answer: a phone that receives them early can read a
perfect score off the network tab. `stripPollPoints` in `backend/internal/api/poll.go`
enforces this on both the WebSocket envelope and the public questions endpoint.

### The TV board

`/g/{code}/board` is a read-only projector view — no player token, no leaderboard
row, no ability to answer. It connects with `?role=board` as a third WebSocket
role alongside `player` and `admin`, and receives the *player-facing* state, so a
TV in the room never reveals points early.

In setup it shows a scannable QR for the join URL and the teams as they arrive.
During a question it hides the options and lights up each team name as they lock
in. On reveal it fills the board in from #5 up to #1, and a standings rail runs
along the bottom for the whole game. Open it from the admin console
("📺 Open TV board") or navigate directly.

### Scheduled start & magic-link login

If the admin sets a **scheduled start** time on a game, the player UX adapts to it. A single threshold drives the behavior:

```
WITHIN_THRESHOLD = scheduledAt − now ≤ 60 min
```

| Surface                               | Within threshold (or no schedule set) | Outside threshold (≥ 60 min away) |
| ------------------------------------- | ------------------------------------- | --------------------------------- |
| Join page (`/g/:code/join`)           | Name + photo only.                    | Name + photo + **optional email**. |
| "Locked in" screen (post-submit)      | Bold wait notice ("If you wait here, you will still participate…"). | Inline pitch to drop an email and get a one-click rejoin link. |
| Profile edit dialog                   | Email field always visible & editable. | Same. |

When a player provides a non-empty email (at join, on the locked-in screen, or via the profile dialog), the backend sends a **magic-link email** in the background. The link reuses the existing admin "impersonate" deep-link format — `https://<PUBLIC_BASE_URL>/impersonate#token=<playerToken>` — so there's no separate one-time-token table; the link is valid as long as the player's token is. If `SMTP_ENABLED=false`, the URL is logged to stdout instead, which is enough for dev/test.

The 60-minute threshold and the relogin pitch are deliberate: a player who joins right before kickoff is already mid-flow and doesn't need the email; a player who pre-joins hours early will close the tab and needs a way back in.

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
│   │   ├── mail/       # SMTP magic-link sender (no-op when SMTP_ENABLED=false)
│   │   └── ws/         # generic websocket hub
│   └── migrations/
│       ├── 0001_init.sql
│       ├── 0002_question_timeout.sql
│       ├── 0003_game_scheduled_at.sql
│       ├── 0004_user_email.sql
│       ├── 0005_orphan_on_user_delete.sql
│       ├── 0006_user_last_seen.sql
│       ├── 0007_images.sql            # images + image_variants tables, photo_image_id FKs
│       ├── 0008_drop_photo_b64.sql    # contract step: drop legacy base64 columns
│       └── 0009_poll_questions.sql    # Company Consensus mode: 'poll' answer type + games.mode
├── frontend/
│   ├── nginx.conf
│   └── src/
│       ├── pages/       # Landing, Join, Setup, Game, Board, Results, Impersonate, Admin*
│       ├── components/  # AppHeader, PhotoPicker, Spotlight, Stepper, ConfirmDialog, ProfileDialog
│       │   └── admin/   # LiveQuestion, PlayersList, QuestionSubmissions, PollQuestions, PollQuestionForm, QuestionImport
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

Run the integration test (boots a Postgres container via `testcontainers-go` and drives a full game over the real WebSocket hub + scoring + reveal — requires a running Docker daemon):

```bash
cd backend && go test -tags=integration -run TestIntegration ./internal/api/
```

Build the frontend without Docker (type-check then bundle):

```bash
cd frontend && npm install && npm run build
```

Hot-reload dev (talks to backend in Docker on `:8080`):

```bash
cd frontend && npm run dev
```

The `oglimmer.sh` helper at the repo root wraps the common loops: `./oglimmer.sh start|stop|status|logs|test` for a local Go binary; `./oglimmer.sh build [-f|-b]` for Docker build + push + `kubectl rollout restart`; `./oglimmer.sh release --bump minor` runs the integration test suite (`-tags=integration`, needs Docker) as a pre-flight, then bumps the frontend `package.json` and Helm `Chart.yaml` together, tags, and pushes — a failing integration test aborts before anything is committed.

## Kubernetes (Helm)

The `helm/trivia` chart deploys backend + frontend + (optional) bundled Postgres behind Traefik 3 with cert-manager-issued TLS. WebSockets ride the same origin over `/ws`; Traefik handles the `Upgrade` handshake transparently, no per-Ingress annotations required.

```bash
# 1. Seal the cluster secrets (one-time, against your kubeseal controller).
#    SMTP_USER / SMTP_PASSWORD are optional — only add them if your relay
#    requires PLAIN auth.
kubectl create secret generic trivia-secret \
  --namespace default --dry-run=client -o yaml \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -hex 16) \
  --from-literal=JWT_SECRET=$(openssl rand -hex 32) \
  --from-literal=ADMIN_PASSWORD=<choose> \
  --from-literal=ANTHROPIC_API_KEY=<sk-ant-...> \
  --from-literal=SMTP_USER=<relay-username> \
  --from-literal=SMTP_PASSWORD=<relay-password> \
  | kubeseal --format yaml > helm/trivia/templates/sealed-secret.yaml

# 2. Install
helm install trivia ./helm/trivia
```

Key `values.yaml` knobs: `publicBaseURL`, `anthropic.model`, `backend.image` / `frontend.image` (default `ghcr.io/oglimmer/trivia-{backend,frontend}:latest`), `postgres.enabled` (toggle off to point at `externalPostgres.host` instead), `postgres.image.tag` (pinned at `16-alpine`; bumping it is a major Postgres upgrade — see the comment in `values.yaml` and don't do it without a dump/restore plan), `ingress.hosts` / `ingress.tls`. The default cert-manager issuer is `oglimmer-com-dns` (DNS-01); change `ingress.annotations.cert-manager.io/cluster-issuer` and `ingress.tls[].secretName` for your own setup. See `helm/trivia/README.md` for more.

#### Enabling SMTP

The `smtp` block in `values.yaml` controls the magic-link sender. Three transport modes are supported, pick the one your relay expects:

```yaml
smtp:
  enabled: true
  host: "smtp.example.com"
  port: 587            # 25 plain | 587 STARTTLS | 465 implicit TLS
  tls: false           # implicit TLS (SMTPS). mutually exclusive with starttls
  starttls: true       # require STARTTLS upgrade
  from: "noreply@example.com"
  replyEmail: "noreply@example.com"
  replyName: "Trivia-Helper"
```

`SMTP_USER` / `SMTP_PASSWORD` come from the sealed secret as `optional` keys — the deployment renders without them when the relay is anonymous. With `smtp.enabled=false` (default), the backend keeps logging the magic-link URL to stdout, which is enough for staging or for verifying the rest of the flow before pointing at a real relay.

## API reference (short)

Public:
- `GET  /api/games/{code}` — basic game info for the join screen, including `scheduledAt` (so the join page can decide whether to ask for an email)
- `POST /api/games/{code}/join` — `{name, photoImageId?, email?}` → `{token, userId, gameId, code}`. A non-empty `email` triggers a magic-link send. `photoImageId` references an image created via `POST /api/images`.
- `GET  /api/me` — `X-Player-Token` header → `{user, game}`
- `PUT  /api/me` — `{name, photoImageId?, email?}` (player edits own profile). Magic-link only fires when `email` is newly set or changes — repeat saves don't spam.
- `GET  /api/games/{code}/users` — list of joined players
- `GET  /api/games/{code}/questions` — questions for the game (`correct` only included once the game is `finished`)
- `PUT  /api/games/{code}/questions` — submit/update the player's question (`photoImageId` required)
- `GET  /api/games/{code}/leaderboard`
- `POST /api/ai/suggest` — `{hint, answerType, photoImageId?}` → `{text, options, correct}`. The backend fetches the `medium` variant of the referenced image and includes it as a vision block in the prompt. The call enables Anthropic's `web_search_20260209` tool (capped at 3 searches) so the model can verify facts before committing them, which makes the request fan out to 60–90 s end-to-end — the HTTP client timeout is 120 s.

Image store:
- `POST /api/images` — multipart `file` (≤ 8 MiB). Server re-encodes to JPEG q=85, generates `thumb` (128 px) and `medium` (640 px) variants, dedupes by sha256, and returns `{id}`.
- `GET  /api/images/{id}` — original bytes.
- `GET  /api/images/{id}/thumb` | `GET  /api/images/{id}/medium` — variants.
- Responses set `Cache-Control: public, max-age=31536000, immutable` and a sha256-based `ETag`; `If-None-Match` returns `304`. No auth — the UUID is the capability.

Admin (`Authorization: Bearer <jwt>`):
- `POST   /api/admin/login` — `{password}` → `{token}`
- `GET    /api/admin/games`, `POST /api/admin/games` (`{code?, name, questionTimeoutSeconds?, scheduledAt?}` — `scheduledAt` is an RFC 3339 timestamp string, omit/null for no schedule)
- `GET    /api/admin/games/{code}`
- `DELETE /api/admin/games/{code}` — drops the game; connected clients get a `gameDeleted` frame
- `POST   /api/admin/games/{code}/state` — `{state: "setup"|"game"|"finished"}`
- `PUT    /api/admin/games/{code}/settings` — `{questionTimeoutSeconds?, scheduledAt?: string|null}` (setup only). Each field is optional and treated independently: omit to leave unchanged, send `null` for `scheduledAt` to clear it.
- `POST   /api/admin/games/{code}/activate` — optional `{questionId}` (else next by sort order)
- `POST   /api/admin/games/{code}/reveal`
- `POST   /api/admin/games/{code}/next`
- `POST   /api/admin/games/{code}/finish`
- `DELETE /api/admin/games/{code}/users/{userId}` (setup only)
- `GET    /api/admin/games/{code}/users/{userId}/impersonate` — returns that player's token + a deep-link the host can hand them
- `DELETE /api/admin/games/{code}/questions/{questionId}` (setup only)

Observability:
- `GET /metrics` — Prometheus exposition. Mounted on the root mux outside CORS/access-log middleware. Requires `Authorization: Bearer $METRICS_TOKEN`; when `METRICS_TOKEN` is unset the endpoint returns 404 (fail-closed). Exposes the standard Go runtime + process collectors plus app metrics under the `trivia_` namespace: `trivia_http_requests_total{method,path,status}` and `trivia_http_request_duration_seconds` (path is the chi route pattern, so URL params don't blow up label cardinality; scoped to `/api` only — `/ws` is excluded because its handler lifetime is the WebSocket session, not request latency), `trivia_http_in_flight_requests`, `trivia_ws_connections{role}`, `trivia_ws_session_duration_seconds{role}` (lifetime of each WebSocket session), `trivia_game_count{state}`, `trivia_game_online_players`, `trivia_game_answers_submitted_total{result}`, `trivia_game_questions_{activated,revealed,auto_closed}_total`, `trivia_ai_suggest_requests_total{result}` + `_duration_seconds`, `trivia_images_{uploaded,orphans_deleted}_total`, and a `trivia_build_info` info-gauge. The Helm chart wires `METRICS_TOKEN` from the sealed secret as an optional key — add a sealed entry to enable scraping in prod.

WebSocket at `/ws`:
- Player: `?token=<playerToken>`
- Admin:  `?role=admin&token=<adminJWT>&code=<gameCode>`

Inbound (from client):
- `{type:"answer", data:{questionId, value}}` — `value` shape depends on answer type (`"yes"|"no"`, integer index, or number).
- `{type:"ping"}` → `{type:"pong"}`.

Outbound (from server):
- `gameState` — the current view, including `questionTimeoutSeconds`, `scheduledAt`, and `serverNow` so the client can compute clock skew for an accurate countdown. Admins additionally get `correct` and `questionsAdmin`.
- `users` — list of joined players (no tokens).
- `presence` — admin only; `{online: [userId, ...]}` of players with at least one live connection. Sent on every player join/leave.
- `playerAnswered` — admin only; fires as each player submits.
- `answerAck` — to the submitting player, with `responseMs`.
- `gameDeleted` — sent right before the host deletes a game.

Connection lifecycle: server pings every 30 s with a 75 s read deadline. The client also closes the socket on `visibilitychange→hidden` / `pagehide` so the player drops from the admin's presence list within a network RTT — without this, a backgrounded mobile tab would appear online for ~75 s after the user left the app. On `visibilitychange→visible` / `pageshow` / `online`, the client reconnects.

## Gaps & next steps

### Deliberate trade-offs

- **Photos stored as BYTEA in Postgres**, in a dedicated `images` / `image_variants` pair of tables (rather than an object store). Content-addressed by sha256 → dedupe; thumb (128 px) + medium (640 px) variants are generated once on upload so the hot path never resizes. The frontend resizes to 1024 px / JPEG q=0.82 before upload to keep uploads cheap on mobile; the server re-encodes and is the authority. Simple to operate, works offline, and `Cache-Control: immutable` + ETag means thumbnails are served from the browser cache after the first hit — but the DB still grows with every unique photo. See `docs/image-architecture.md` for the design; the S3/MinIO escape hatch is listed under "If I were to keep building" below.
- **Single admin via env var**, not a `users` table. Matches "an admin has to authenticate" without overbuilding.
- **Randomized question order is set once on entering game mode**, not re-shuffled mid-game.
- **Players see only their own answer ack** during a round, not who else has answered (admin sees the live tally). Avoids social pressure / spoilers.
- **All answers shown on reveal**, not just the correct ones — more transparent and lets losers see how close they were.
- **Number scoring is rank-based**, not threshold-based. A whole field of bad guesses still produces a podium; lone perfect guesses still win cleanly because of the exact-tolerance branch.
- **Magic-link login reuses the player token directly** (`/impersonate#token=…`) rather than issuing a separate one-time token. No new table, no expiry logic — the link is exactly as valuable as the cookie that already lives in the player's other browser. Trade-off: forwarding the email forwards the session. Acceptable for a casual party app; not what you'd ship for a payments product.

### Real gaps & nice-to-haves

- **No HTTPS / WSS in the compose setup**. Fine for LAN play; the Helm chart terminates TLS at ingress for prod.
- **No rate limiting** on `/api/games/{code}/join`, `/api/ai/suggest`, or magic-link sends triggered via `PUT /api/me` — all spammable. Add a per-IP / per-game token bucket if exposing publicly.
- **JWT secret defaults to a dev value** if `JWT_SECRET` is unset. Loud-fail on startup would be safer.
- **Migrations are idempotent `*.sql` files**, not versioned with a tool like `goose`/`atlas`. Adding a column today is fine; renaming one needs a tool.
- **No game/image cleanup**. Old games linger forever; deleting a game doesn't touch its players' photos because the FK on `images` is `ON DELETE SET NULL` (per the `docs/image-architecture.md` design). Easy wins: a daily job that drops games older than N days, plus a periodic orphan sweep that deletes `images` rows no longer referenced by `users.photo_image_id` or `questions.photo_image_id` (CASCADE handles `image_variants`).
- **No host-side reorder**: the admin can delete a question in setup, but ordering is still randomized once on transition to `game`.
- **No "rewind"**: admin can only move forward through questions, can't re-open a revealed one.
- **PWA basics missing**: no manifest, no service worker, no offline cache. App still works fine in a normal mobile browser.
- **AI suggestion** returns the first JSON block found in the last text content block (web search produces interleaved tool-use / tool-result blocks); on rare malformed output the user gets the raw error. Could retry or wrap in a structured-output flow.
- **Accessibility**: the option buttons have keyboard focus and decent contrast, but there's no formal WCAG pass — screen-reader labels on the iconic buttons (camera, library, ✓, ✕) would help.
- **The podium reveal in `Results.vue` is one route with phases** rather than three separate URLs. Functionally identical to "different pages" but the back button skips the whole sequence.

### Capacity notes

Quick assessment of the default Helm resource limits (`backend`: 500m CPU / 512 Mi mem, `postgres`: 500m CPU / 512 Mi mem, `pgxpool.MaxConns = 25`) against an 80-player game:

- **Should work.** WS hub is 81 connections × 2 goroutines + a 256-entry send buffer each — tens of MB at worst. A reveal marshals one `gameState` envelope (~30–60 KB JSON with 80 answers + leaderboard) and fans out to 81 clients; sub-ms CPU.
- **Tightest path:** each `handleAnswer` does 3 DB queries (`GameByID`, `QuestionByID`, `SaveAnswer`). 80 simultaneous answers → 240 queries against a pool of 25 ≈ 25–50 ms of queueing. Fine at 80, first thing to hurt past ~200.
- **Number-question rescore** (`rescoreNumberAnswers` in `backend/internal/api/server.go`) issues N sequential `UpdateAnswerScore` writes on reveal. 80 × ~5 ms ≈ 400 ms before the reveal broadcast goes out.
- **Cheap headroom upgrades** if pushing toward 150–200 players: (1) raise `MaxConns` to ~50 in `backend/internal/db/db.go`; (2) batch the number-question rescore into a single UPDATE; (3) consider an in-memory cache for `GameByID` / `QuestionByID` on the answer hot path.

### If I were to keep building

1. Move the image bytes off Postgres to S3/MinIO (metadata + dedupe stay in `images` / `image_variants`); the serving endpoint becomes a redirect or signed-URL issuer. Only worth it once DB volume gets uncomfortable.
2. WebP/AVIF variants alongside the JPEG thumb/medium (~30% smaller at equal quality), negotiated via the `Accept` header. Doubles per-image storage but cuts every image fetch.
3. Blurhash / LQIP placeholders — a ~20-char hash stored on each `images` row, returned alongside `photoImageId`, rendered as a blurred gradient while the real thumbnail loads. Perceived-perf polish, not bytes-on-the-wire.
4. Per-game settings panel: point shape, reveal autoplay (the answer window is already configurable).
5. Multi-admin / SSO if this ever needs to scale past one host.
6. PWA install + offline page so players don't lose state if their connection blips.
