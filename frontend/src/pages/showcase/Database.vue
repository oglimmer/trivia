<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 03</span>
      <h1 class="hero__title">Database /<br /><em>plain Postgres, plain SQL</em></h1>
      <p class="hero__subtitle">
        No ORM, no DSL — just <code>pgx</code> + hand-written SQL behind a thin
        repository layer. Migrations are flat <code>.sql</code> files applied
        in name order at boot.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>Postgres 16, single instance — see <RouterLink to="/developers-showcase/deployment">deployment</RouterLink>.</li>
        <li><code>github.com/jackc/pgx/v5/pgxpool</code> as the driver; no <code>database/sql</code> wrapper.</li>
        <li>Repository methods hang off a <code>*DB</code> struct in <code>internal/db</code> — one file per aggregate.</li>
        <li>Migrations are flat <code>.sql</code> files in <code>backend/migrations</code>, run alphabetically by the binary at startup.</li>
        <li>Answers/options/correct live in <code>JSONB</code> because the shape depends on the answer type.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/db/db.go</code> — pool config, <code>Migrate</code>.</li>
        <li><code>backend/internal/db/models.go</code> — Go structs that match the row shape.</li>
        <li><code>backend/internal/db/games.go</code> · <code>users.go</code> · <code>questions.go</code> · <code>answers.go</code>.</li>
        <li><code>backend/migrations/*.sql</code> — schema evolution, numbered and append-only.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Connection pool</h2>
      <pre class="api-code">// backend/internal/db/db.go
cfg.MaxConns          = 25
cfg.MinConns          = 5
cfg.MaxConnLifetime   = 30 * time.Minute
cfg.MaxConnIdleTime   = 5  * time.Minute
cfg.HealthCheckPeriod = 1  * time.Minute</pre>
      <p>
        25 connections is comfortable for a single-pod backend talking to a
        single-pod Postgres. The lifetime cap keeps long-lived TCP sessions
        from drifting into a broken state after a Postgres restart or an
        intermediate proxy idle-timeout.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Migrations</h2>
      <p>
        The runtime reads every <code>.sql</code> in the migrations folder,
        sorts by filename, and execs each one. Statements use
        <code>IF NOT EXISTS</code> / <code>IF EXISTS</code> so re-runs are
        idempotent — there is no migration tracking table.
      </p>
      <pre class="api-code">// backend/internal/db/db.go
func (d *DB) Migrate(ctx context.Context, dir string) error {
    entries, _ := os.ReadDir(dir)
    names := []string{}
    for _, e := range entries {
        if !e.IsDir() &amp;&amp; strings.HasSuffix(e.Name(), ".sql") {
            names = append(names, e.Name())
        }
    }
    sort.Strings(names)
    for _, n := range names {
        b, _ := os.ReadFile(dir + "/" + n)
        if _, err := d.Pool.Exec(ctx, string(b)); err != nil {
            return fmt.Errorf("migration %s: %w", n, err)
        }
    }
    return nil
}</pre>
      <p>
        <strong>Convention:</strong> never edit a shipped migration. The next
        change goes in the next-numbered file. The current chain:
      </p>
      <ul>
        <li><code>0001_init.sql</code> — games, users, questions, answers.</li>
        <li><code>0002_question_timeout.sql</code> — per-game timeout.</li>
        <li><code>0003_game_scheduled_at.sql</code> — optional start time.</li>
        <li><code>0004_user_email.sql</code> — magic-link target.</li>
        <li><code>0005_orphan_on_user_delete.sql</code> — keep Qs/answers when an author is removed (FK → SET NULL).</li>
        <li><code>0006_user_last_seen.sql</code> — drives the idle-prune sweep.</li>
        <li><code>0007_images.sql</code> — images + variants tables; FKs from users/questions.</li>
        <li><code>0008_drop_photo_b64.sql</code> — old inline-base64 column gone.</li>
        <li><code>0009_poll_questions.sql</code> — Company Consensus: the <code>poll</code> answer type, <code>games.mode</code>, <code>games.hide_leaderboard_tail</code>.</li>
        <li><code>0009_user_name_unique_per_game.sql</code> — case-insensitive partial index.</li>
        <li><code>0010_question_votes.sql</code> — one best-question vote per player per game.</li>
      </ul>
      <p>
        Note the two <code>0009</code> files: they landed on separate branches
        and were both merged. Sorting is by full filename, so
        <code>poll_questions</code> runs before
        <code>user_name_unique_per_game</code> every time, and they touch
        different tables — but the collision is a warning, not a pattern. Check
        the highest number before naming a new one.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>JSONB columns</h2>
      <p>
        The shape of <code>options</code>, <code>correct</code>, and an
        answer's <code>answer</code> depends on <code>answer_type</code>:
      </p>
      <pre class="api-code">-- migration 0001
options       JSONB NOT NULL DEFAULT '[]'::jsonb,
correct       JSONB NOT NULL,
-- and in answers:
answer        JSONB NOT NULL,</pre>
      <p>
        Rather than three columns per shape (a "string-correct", an
        "int-correct", a "number-correct"), each is a single JSONB. The Go
        side keeps them as <code>json.RawMessage</code> and only unmarshals at
        scoring time, where the type is known:
      </p>
      <pre class="api-code">// backend/internal/db/models.go
type Question struct {
    // ...
    AnswerType string          `json:"answerType"`
    Options    json.RawMessage `json:"options"`
    Correct    json.RawMessage `json:"correct,omitempty"`
    // ...
}</pre>
      <p>
        The scoring code (<RouterLink to="/developers-showcase/scoring">showcase</RouterLink>)
        decodes into <code>string</code>, <code>int</code>, or
        <code>float64</code> depending on the answer type — without any
        per-column polymorphism on the DB side.
      </p>
      <p>
        Adding the <code>poll</code> type is what this design was for. Its
        <code>options</code> are objects rather than strings, and it has no
        correct answer at all — so it stores the JSON literal
        <code>null</code>, which satisfies the <code>NOT NULL</code> column
        without inventing a sentinel:
      </p>
      <pre class="api-code">-- a poll question, as stored
answer_type = 'poll'
options     = '[{"text":"Tea","points":27},{"text":"Coffee","points":41}, ...]'
correct     = 'null'</pre>
      <p>
        Migration <code>0009_poll_questions.sql</code> is correspondingly
        small: widen the <code>answer_type</code> check constraint, add two
        columns to <code>games</code>. No new table, no shape change to
        <code>answers</code> — a poll submission is an option index, which is
        the same thing a <code>choice</code> answer already stored.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Cascade graph</h2>
      <p>The FK choices encode the lifecycle:</p>
      <pre class="api-code">games        ──┬─→ users      (ON DELETE CASCADE)
               ├─→ questions  (ON DELETE CASCADE)
               └─→ answers    (via question)

users        ──┬─→ questions  (ON DELETE SET NULL)   ← migration 0005
               └─→ answers    (ON DELETE SET NULL)

images       ──┬─→ users.photo_image_id     (ON DELETE SET NULL)
               └─→ questions.photo_image_id (ON DELETE SET NULL)

image_variants ─→ images       (ON DELETE CASCADE)</pre>
      <p>
        Deleting a game wipes the whole subtree in one statement. Removing a
        single player keeps their question in the set (orphaned, admin can
        decide whether to delete it). Deleting an image never cascades into a
        user or question — see <RouterLink to="/developers-showcase/images">images showcase</RouterLink>.
      </p>
      <p>
        <code>questions.user_id</code> being nullable turned out to carry a
        second meaning. It was added so an author could be removed without
        taking their question with them; Company Consensus questions are born
        with it <code>NULL</code>, because the host wrote them and no player
        owns them. That single field is what the admin editor checks before
        allowing an edit — a question with an author belongs to that author.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The repository pattern</h2>
      <p>
        Every query is hand-rolled and lives next to the aggregate it touches.
        Errors that callers want to branch on are mapped to package-level
        sentinels:
      </p>
      <pre class="api-code">// backend/internal/db/models.go
var (
    ErrNotFound  = errors.New("not found")
    ErrNameTaken = errors.New("name already taken")
)</pre>
      <p>
        A good example of the mapping is the name-uniqueness flow. The DB
        enforces it via the partial index from migration 0009; the repo
        translates the resulting <code>SQLSTATE 23505</code> into the sentinel,
        which the HTTP handler then renders as a 409:
      </p>
      <pre class="api-code">// backend/internal/db/users.go
const uniqueViolationName = "uq_users_game_name_lower"

func mapNameTaken(err error) error {
    if err == nil { return nil }
    var pgErr *pgconn.PgError
    if errors.As(err, &amp;pgErr) &amp;&amp; pgErr.Code == "23505" &amp;&amp;
        pgErr.ConstraintName == uniqueViolationName {
        return ErrNameTaken
    }
    return err
}</pre>
      <p>
        The handler then knows nothing about constraint names — just sentinel
        equality.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>No ORM.</strong> The query set is small and bounded; the cost
          of an ORM (mystery N+1s, mapping cliffs, custom expression DSL) is
          not paid for by the bench it would buy.
        </li>
        <li>
          <strong>No migration tracking table.</strong> Idempotent
          <code>IF NOT EXISTS</code> statements suffice for the scale here. If
          a migration ever needs a non-idempotent transform, switch to
          <code>golang-migrate</code> or similar.
        </li>
        <li>
          <strong>Migrations run at app startup.</strong> Two backends starting
          simultaneously will both try to migrate; idempotency is what makes
          that safe. For larger deployments, gate this with a Helm pre-install
          job.
        </li>
        <li>
          <strong>UUID primary keys.</strong> <code>gen_random_uuid()</code> via
          <code>pgcrypto</code> (built in to Postgres ≥ 13). No
          collisions to worry about across distributed clients.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/websocket" class="btn-link">Next: WebSocket →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
