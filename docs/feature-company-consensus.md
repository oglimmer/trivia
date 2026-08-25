# Feature spec: Company Consensus mode (IRL Dubrovnik ice breaker)

**Status:** implemented — see "Decisions taken" below
**Target event:** IRL Dubrovnik, Monday evening
**Author:** drafted with Claude from Oli's brief

## 1. Verdict

Yes — this works on the existing app, but it is a **new game mode**, not a config change.

Roughly 70% of what the format needs already exists and is battle-tested from Vienna:
the game code + join flow, the host-driven round loop with an answer window and
auto-close, the WebSocket live state, the leaderboard and podium finish, admin auth,
and the whole deployment story. What does not exist is the part that makes this
Company Consensus rather than trivia: **there is no notion of an answer that is
"partly right"**, and **the host cannot author the questions** — today every
question is written by a player, one each.

Four real gaps, listed in section 3. None of them are architectural; they are
additive. Estimate in section 10.

## 2. What already fits, unchanged

| Requirement (from the brief)              | Existing support |
| ----------------------------------------- | ---------------- |
| One person per team scans a QR to join     | `/g/:code/join`, name + optional photo → player token. Use the **team name** as the player name. |
| ~10 teams                                  | No limit; Vienna ran comfortably above this. |
| Host runs the round, everyone answers live | `activate → reveal → next` in `internal/api/admin.go`, broadcast over the WS hub. |
| Answer window with auto-close              | `question_timeout_seconds` per game (5–600 s), timer re-armed on backend restart. |
| One clear winner on total points           | `db.Leaderboard`, `Spotlight.vue` podium (3rd → 2nd → 1st, then full ladder). |
| Reconnect / phone dies / tab closed        | Player token + magic-link relogin, WS replays state on reconnect. |
| 15 questions                                | No cap. Order is shuffled on `setup → game`. |

**Teams map onto the existing `users` table with no schema change.** A "user" is a
team: one device, one token, one row on the leaderboard. Team rosters (the 6–7
humans) are not modelled and do not need to be — put them on paper.

## 3. Gaps

### 3.1 The host cannot author questions

Questions are player-submitted: `PUT /api/games/{code}/questions` requires a player
token, requires `game.state = 'setup'`, requires a photo, and `questions` carries a
`UNIQUE (game_id, user_id)` — one question per player, forever.

**Needed:** an admin bulk-import endpoint that creates all 15 questions at once from
pasted JSON, with `user_id = NULL` (already nullable since migration `0005`, and
Postgres lets `NULL` repeat under a `UNIQUE`, so the constraint does not need to
change) and no photo.

### 3.2 Scoring is binary; this format is not

`game.JudgeAnswer` returns `(isCorrect bool, points int)` — one option is right and
carries the whole base score, everything else is zero. Company Consensus needs **five
options, each carrying its own point value**, all of them "correct".

**Needed:** a new answer type, `poll`, where the point value travels with the option.

### 3.3 The option cap is 4

`validateQuestion` rejects a `choice` question with more than 4 options, and
`Setup.vue:136` hides the "+ Add option" button at 4. The format needs 5.

### 3.4 There is no big-screen view

There is an admin console (`AdminGame.vue`) and there is the player's phone. There is
nothing designed to be projected. The brief asks for a live leaderboard on a TV, and
the reveal — watching the board fill in from #5 up to #1 — is the best moment of the
format, so it deserves a real screen.

**Needed:** a read-only `/g/:code/board` page.

## 4. Design decisions (and why)

These are recommendations, not settled. Section 9 lists what I need from you.

### 4.1 Drop the time bonus in this mode

Today `timeBonus = base × 0.5 × max(0, 1 − responseMs / windowMs)`, so the fastest
answer earns up to 1.5× base. That is right for solo trivia and **wrong here**: a
team of 6–7 is supposed to argue about what the crowd said. A decaying clock punishes
the discussion that is the entire point of the ice breaker, and it hands the win to
whichever team has the twitchiest phone operator.

**Recommendation:** `poll` questions score exactly the option's point value. No bonus.
Pair that with a longer window — **60–90 s** instead of the 30 s default.

### 4.2 …but keep response time as a hidden tiebreaker

Removing the bonus makes ties much more likely: with 15 questions drawing from a small
set of round point values, two teams landing on the same total is a live possibility,
and the brief asks for *one clear winner*. The current sort is
`ORDER BY points DESC, u.name ASC` — alphabetical, which is not a defensible way to
lose a party game.

**Recommendation:** break ties on `SUM(response_ms) ASC`. It is already recorded on
every answer, costs one clause in `db.Leaderboard`, and is invisible unless it is
needed. If you would rather settle it in the room, the alternative is a host-run
sudden-death 16th question — but that needs a UI affordance that does not exist, so I
would take the cheap version.

### 4.3 Points come from the survey, and the host controls them

Rather than deriving points from a formula, the imported JSON carries the number per
option — e.g. the top answer given by 41 of 80 respondents is worth 41 points, or you
round to 40/30/20/10/5. Both work; the app just stores what you upload. Raw counts are
more honest and let the board show "41 people said this", which reads well on a TV.

**Recommendation:** upload raw respondent counts, and let the board display them
as both the count and the score, because in this format they are the same thing.

### 4.4 Turn off the leaderboard suspense tail for this game

`leaderboardSuspenseTail = 3` in `helpers.go` hides the running leaderboard from
players for the last 3 questions to keep the finish tense. You explicitly want a
**live** leaderboard on the TV, so for this mode it needs to be off — or at least a
per-game toggle. I would make it a game setting rather than hard-coding the new
behavior, since the suspense tail is genuinely good for the classic mode.

### 4.5 No strikes, no free text at game time

Real Company Consensus has teams calling out free-text answers and getting X'd. The brief
describes something simpler — teams pick one of 5 shown options — and that is the
right call for 10 teams on phones: it is unambiguous, self-scoring, and needs no
judge. This spec builds the simple version. Free-text-at-the-table would be a much
bigger feature and I would not attempt it before Monday.

## 5. Data model

Minimal. One migration.

```sql
-- 0009_poll_questions.sql
ALTER TABLE questions DROP CONSTRAINT questions_answer_type_check;
ALTER TABLE questions ADD CONSTRAINT questions_answer_type_check
    CHECK (answer_type IN ('yesno','choice','number','poll'));

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'classic'
        CHECK (mode IN ('classic','poll')),
    ADD COLUMN IF NOT EXISTS hide_leaderboard_tail BOOLEAN NOT NULL DEFAULT true;
```

For `answer_type = 'poll'`, the existing `options JSONB` column holds objects instead
of strings — no new column needed:

```json
[
  {"text": "Pizza",  "points": 41},
  {"text": "Sushi",  "points": 22},
  {"text": "Burger", "points": 11},
  {"text": "Pasta",  "points": 7},
  {"text": "Salad",  "points": 4}
]
```

`correct` is `NOT NULL`, so `poll` rows store the JSON literal `null` (a valid
non-NULL `JSONB` value). Nothing else changes: `answers.points` already carries an
arbitrary int, and `answers.is_correct` becomes "scored above zero", which is true
for every one of the five options. Existing games and the classic flow are untouched.

`photo_image_id` is already nullable — imported questions simply have none, and the
board/phone render a text-only card.

## 6. Backend

| Change | File |
| ------ | ---- |
| `POST /api/admin/games/{code}/questions/import` — accepts the JSON in §8, replaces the game's question set in one transaction, `user_id NULL`, `sort_order` from array position | new handler in `admin.go`, route in `routes.go` |
| `mode` + `hide_leaderboard_tail` on create and `PUT .../settings` | `admin.go` |
| `poll` branch in `JudgeAnswer` → `points = options[i].points`, `isCorrect = points > 0`, no time bonus | `internal/game/scoring.go` |
| `OptionCount` handles an array of objects | `internal/game/scoring.go` |
| `validateQuestion`: allow `poll` (exactly 5 options, each `text` non-empty and `points >= 0`); raise the `choice` cap to 5 | `internal/api/player.go` |
| `inLeaderboardSuspense` respects `game.hide_leaderboard_tail` | `internal/api/helpers.go` |
| Leaderboard tiebreak on `SUM(response_ms) ASC` | `internal/db/answers.go` |
| Reveal broadcast carries the full option→points board for `poll` | `internal/api/broadcast.go` |

Scoring is pure and unit-tested (`scoring_test.go`) — the `poll` branch should land
with tests alongside the existing table-driven cases. The integration test in
`internal/api/integration_test.go` drives a full game over the real hub; a `poll`
variant of it is the cheapest way to be confident before Monday.

**Import must be idempotent and re-runnable while `state = 'setup'`.** You will paste
the wrong points at least once; re-importing should wipe and replace, not append.
Reject import once the game has started.

## 7. Frontend

| Change | File |
| ------ | ---- |
| Render 5 poll options as tappable cards; no "correct/incorrect" language on the result card — show *"You picked Sushi — 22 points. Top answer was Pizza (41)."* | `pages/Game.vue` |
| Skip the question editor when `game.mode = 'poll'`; setup becomes a plain lobby (team name + photo, waiting list) | `pages/Setup.vue` |
| Import panel: paste JSON, validate, show the parsed 15 questions for confirmation | `pages/AdminGame.vue` |
| **New:** `/g/:code/board` — projector view. Big question text, live "6 / 10 teams answered", then on reveal the five answers filling in from #5 to #1 with their points, then the leaderboard. Reuses the existing WS store and `Leaderboard.vue`. | new `pages/Board.vue` |
| `'poll'` in the question type union | `services/api/*.ts` |

The board page is the only genuinely new screen. Everything else is a branch inside a
page that already exists.

Note the board must join the WS **without a player token** (it is a TV, not a
player, and must not appear on the leaderboard). Today the non-admin WS path
requires a valid player token. Cleanest fix: let the board connect with
`?role=board&code=<code>`, read-only, no `userID` — a third role in `internal/ws`
alongside `player` and `admin`.

## 8. Import format

```json
{
  "questions": [
    {
      "text": "Name something you always forget to pack.",
      "answers": [
        {"text": "Toothbrush",  "points": 41},
        {"text": "Charger",     "points": 22},
        {"text": "Socks",       "points": 11},
        {"text": "Sunscreen",   "points": 7},
        {"text": "Passport",    "points": 4}
      ]
    }
  ]
}
```

Validation: 1–50 questions, exactly 5 answers each, non-empty text, `points >= 0`.
The options are shuffled per-question on import so the highest-scoring answer is not
always listed first — otherwise the game is trivially solvable by picking the top row
every time. **This is essential and easy to forget.**

Question order is already shuffled on `setup → game` by `RandomizeQuestionOrder`; for
a host-authored set you probably want the order you uploaded. Recommendation: skip the
shuffle when `mode = 'poll'`.

## 9. Decisions taken during implementation

The open questions below were answered before the build. Recording the answers
here so the spec matches what shipped.

1. **Time bonus: kept.** Poll answers score `surveyCount + timeBonus`, the same
   shape as classic. §4.1 argued for dropping it; the call was to keep it.
2. **The bonus window was broken and is now fixed.** `timeBonus` decayed over a
   hardcoded `AnswerWindowMs = 30_000` and never saw the game's configured
   window — the README's claim to the contrary was wrong. With a 90 s question
   that meant the bonus died a third of the way in, so every team that actually
   discussed the answer scored flat. The game's `question_timeout_seconds` is
   now threaded into `JudgeAnswer` and `ScoreNumberAnswers`. **This changes
   classic games too**: one configured with a non-30 s window previously had its
   bonus expire at 30 s and now decays across the whole window.
3. **No tiebreak was added.** §4.2 proposed one because dropping the bonus makes
   ties likely; with the bonus kept, scores carry enough entropy that it is not
   worth the change to `db.Leaderboard`.
4. **Board shows named teams**, lighting each up as it locks in, rather than an
   anonymous count.
5. **Imported questions are text-only.** `photo_image_id` stays NULL and every
   surface that renders a question photo now guards on it.
6. **The `choice` cap stays at 4.** §3.3 called for raising it to 5, but that was
   written before `poll` became its own answer type — poll carries its own rule
   (exactly 5) and never touches the `choice` path.
7. **JSON is no longer the authoring surface.** The first cut shipped only the
   bulk import from §8, which is a poor way to write and tweak 15 questions. The
   admin console now has a normal add / edit / remove / list editor, plus ↑ / ↓
   to arrange the running order — a poll set is played in the order the host
   built it, so without reorder the order would be fixed at insertion time. The
   JSON import survives, collapsed, for pasting a whole survey at once. Both
   paths share one validator (`buildPollQuestion`) so their rules cannot drift.
8. **`qrcode` was added as a dependency** so the board renders a real scannable
   code. It is in `package.json` but **not installed**: `node_modules` is
   bind-mounted from macOS, and installing from the Linux container would
   overwrite the darwin native binaries in it. Run `npm install` in `frontend/`
   before building or committing.

## 10. Original open questions

1. **Points as raw survey counts, or rounded 40/30/20/10/5?** Affects nothing in the
   code — just tell me which and I will format the import that way.
2. **Answer window:** I am assuming 90 s per question. With 15 questions that is
   ~22 min of answering plus reveal time, call it 35–40 min of game. Shorter if you
   want it tighter.
3. **Time bonus off, hidden time tiebreak on** — confirm §4.1 and §4.2.
4. **Photos on questions?** Currently mandatory for player-written questions,
   proposed optional for imported ones. Do you want the option to attach an image to
   some questions, or is text-only fine for all 15?
5. **Should the board show which team answered, live?** Anonymous "7 / 10 answered"
   is safer; named is more fun. Easy either way.

## 11. Effort

Backend is about a day: the migration and scoring branch are small and well-isolated,
the import endpoint is routine, and most of the time goes into tests. Frontend is
about a day, dominated by the board page. Call it **2 days plus a rehearsal**, and the
rehearsal is not optional — run a full 15-question game with 3–4 phones against the
real deployment before Monday, because the failure modes in this app have always been
about reconnects and the host's finger on the ▶ button, not about scoring.

Suggested order, so there is always something shippable:

1. Migration + `poll` scoring + tests
2. Import endpoint + admin paste panel — at this point you can play the whole game on
   phones with the host console, no TV
3. Player-side poll rendering and reveal copy
4. Board page
5. Tiebreak, suspense toggle, polish

Steps 1–3 are the ice breaker. Step 4 is what makes it feel like a show.

## 12. Out of scope

- The survey itself. Collect it however you like (Google Form works); clustering the
  free-text replies into the top 5 is a Claude conversation, not an app feature. The
  app's contract starts at the JSON in §8.
- Team rosters, per-person scoring, mid-game team changes.
- Strikes, free-text answers at the table, head-to-head "face-off" rounds.
- Anything that changes classic mode behavior. Every change here is behind
  `mode = 'poll'` or a default-preserving setting.
