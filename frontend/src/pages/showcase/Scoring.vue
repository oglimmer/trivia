<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 05</span>
      <h1 class="hero__title">Scoring /<br /><em>fast first, close enough later</em></h1>
      <p class="hero__subtitle">
        Yes/no, choice and poll answers score the moment they arrive. Number
        answers score at reveal time, ranked against the whole field.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>Base points scale with the difficulty floor of the answer type.</li>
        <li>A linearly-decaying time bonus rewards fast correct answers.</li>
        <li>Yes/no, choice &amp; poll → graded individually as they arrive (<code>JudgeAnswer</code>).</li>
        <li>Number → graded as a field at reveal time (<code>ScoreNumberAnswers</code>).</li>
        <li>Poll is the odd one out: there is no correct answer, and the base comes from the data rather than a formula.</li>
        <li>All scoring lives in one file with one dependency: the Go standard library.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/game/scoring.go</code> — every scoring rule.</li>
        <li><code>backend/internal/game/scoring_test.go</code> — table-driven coverage.</li>
        <li><code>backend/internal/game/poll_test.go</code> — the Company Consensus cases.</li>
        <li><code>backend/internal/api/poll.go</code> — building and blinding poll options.</li>
        <li><code>backend/internal/api/ws.go</code> — calls <code>JudgeAnswer</code> on incoming answers.</li>
        <li><code>backend/internal/api/server.go</code> — calls <code>ScoreNumberAnswers</code> at reveal / auto-close.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Base points</h2>
      <p>
        The base is derived purely from the answer type and option count.
        Reasoning: more options or a more open answer space ⇒ harder ⇒ more
        points.
      </p>
      <pre class="api-code">// backend/internal/game/scoring.go
//   yes/no                 100
//   choice with 2 options  100
//   choice with 3 options  200
//   choice with 4 options  300  (+100 per option above 4)
//   number                 300 (open-ended)</pre>
      <p>
        <code>poll</code> never reaches <code>basePoints</code>. A Company
        Consensus option carries its own value — the number of survey
        respondents who gave that answer — so the base is looked up per option
        instead of derived. There is no difficulty floor to reason about,
        because there is no right answer to be wrong about.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Time bonus</h2>
      <p>
        Correct yes/no, choice and poll answers get an extra
        <code>0..base/2</code> that decays linearly to 0 at the end of the
        answer window.
      </p>
      <pre class="api-code">// The window is the game's own questionTimeoutSeconds, passed in by the
// caller. A 90 s question therefore decays over 90 s, not over a fixed 30.
const AnswerWindowMs = 30_000   // fallback when the caller passes 0

func timeBonus(base, responseMs, windowMs int) int {
    if responseMs &lt; 0 { responseMs = 0 }
    w := effectiveWindowMs(windowMs)
    if responseMs &gt;= w { return 0 }
    frac := 1.0 - float64(responseMs)/float64(w)
    return int(math.Round(float64(base) * 0.5 * frac))
}</pre>
      <p>
        So on a 30 s game, a 2-option choice answered at 0 ms is worth
        <code>100 + 50 = 150</code>. Answered at 15 s, <code>100 + 25 = 125</code>.
        Answered at 30 s+, <code>100</code>. Answered <em>after</em> the
        per-question timeout, 0 — the answer is rejected at the WebSocket layer
        before scoring even runs.
      </p>
      <p>
        Passing the window in rather than reading a constant is what let poll
        games ship with a much longer default timeout. A team of six needs time
        to argue, and a bonus that had already decayed to zero by the time they
        agreed would have made speed meaningless.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Yes/no, choice &amp; poll (immediate)</h2>
      <p>
        All three run through <code>JudgeAnswer</code> in the same WebSocket
        message handler that persists the answer.
      </p>
      <pre class="api-code">func JudgeAnswer(answerType string, optionCount int,
                 options, correct, answer json.RawMessage,
                 responseMs, windowMs int) (isCorrect bool, points int) {
    base := basePoints(answerType, optionCount)
    switch answerType {
    case "yesno":
        var c, a string
        _ = json.Unmarshal(correct, &amp;c)
        _ = json.Unmarshal(answer, &amp;a)
        if normYesNo(c) == normYesNo(a) {
            return true, base + timeBonus(base, responseMs, windowMs)
        }
    case "choice":
        var c, a int
        if json.Unmarshal(correct, &amp;c) != nil { return false, 0 }
        if json.Unmarshal(answer,  &amp;a) != nil { return false, 0 }
        if c == a {
            return true, base + timeBonus(base, responseMs, windowMs)
        }
    case "poll":
        // Every option scores; the value is the survey count for that answer.
        opts := ParsePollOptions(options)
        var a int
        if json.Unmarshal(answer, &amp;a) != nil { return false, 0 }
        if a &lt; 0 || a &gt;= len(opts)           { return false, 0 }
        pts := opts[a].Points
        if pts &lt;= 0                          { return false, 0 }
        return true, pts + timeBonus(pts, responseMs, windowMs)
    case "number":
        // Deferred: ScoreNumberAnswers handles this at reveal time.
        return false, 0
    }
    return false, 0
}</pre>
      <p>
        Note <code>normYesNo</code> — the wire format accepts
        <code>"yes" / "Yes" / "YES" / "y" / "Y" / "true"</code> etc. and
        normalises to <code>"yes"</code>. That makes the API tolerant of
        clients without giving up cheap equality on the server.
      </p>
      <p>
        The <code>options</code> parameter exists only for poll. Adding it
        widened the signature for every caller, which is the honest cost of
        letting a question type carry its own point table instead of deriving
        one. The alternative — a second judging entry point — would have split
        the "did this answer arrive in time" rules across two files.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>What <code>isCorrect</code> means in a poll</h2>
      <p>
        Nothing about correctness, which is why the field is the sharpest edge
        in this format. A poll question has no right answer, so
        <code>isCorrect</code> is repurposed to mean "landed on the board at
        all" — true for any option worth more than zero. It drives the ✓ count
        on the leaderboard; the points come from the option.
      </p>
      <p>
        An option worth <code>0</code> returns <code>false, 0</code>. That is
        deliberate: a survey answer nobody gave is the one wrong answer a poll
        can have.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Hiding the points until the reveal</h2>
      <p>
        For every other question type the answer is a separate field from the
        options. For poll they are the same thing: knowing that "Coffee" is
        worth 41 <em>is</em> knowing the answer. So the option points are
        stripped on the way out unless the caller is the admin or the question
        is already revealed.
      </p>
      <pre class="api-code">// backend/internal/api/poll.go
func stripPollPoints(answerType string, options json.RawMessage) json.RawMessage {
    if answerType != "poll" { return options }
    opts := game.ParsePollOptions(options)
    if opts == nil { return json.RawMessage("[]") }
    blind := make([]map[string]string, len(opts))
    for i, o := range opts {
        blind[i] = map[string]string{"text": o.Text}
    }
    // ... marshal
}</pre>
      <p>
        It rebuilds the objects rather than deleting a key, so there is no path
        where a field survives by accident. The board connection gets the
        player-facing view for the same reason: a TV nobody is holding is still
        a screen everybody can read.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Number answers (deferred, ranked)</h2>
      <p>
        Number questions can't be graded at submit time — "is 1889 a good
        guess?" depends on everyone else's guess. So
        <code>JudgeAnswer</code> stores 0 points for them, and the real
        scoring runs at reveal:
      </p>
      <pre class="api-code">// backend/internal/api/server.go (excerpt)
func (s *Server) rescoreNumberAnswers(ctx context.Context, questionID string) error {
    q, _ := s.DB.QuestionByID(ctx, questionID)
    if q.AnswerType != "number" { return nil }
    ans, _ := s.DB.AnswersForQuestion(ctx, questionID)
    inputs := make([]game.NumberAnswer, len(ans))
    for i, a := range ans {
        inputs[i] = game.NumberAnswer{
            UserID: a.UserID, Answer: a.Answer, ResponseMs: a.ResponseMs,
        }
    }
    windowMs := 0
    if g, gerr := s.DB.GameByID(ctx, q.GameID); gerr == nil &amp;&amp; g != nil {
        windowMs = g.QuestionTimeoutSeconds * 1000
    }
    for _, sc := range game.ScoreNumberAnswers(q.Correct, inputs, windowMs) {
        _ = s.DB.UpdateAnswerScore(ctx, questionID, sc.UserID, sc.IsCorrect, sc.Points)
    }
    return nil
}</pre>
      <p>
        Two reasons this gets called: the admin clicked "Reveal", or the
        per-question timer auto-closed the question.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The ranking</h2>
      <pre class="api-code">// backend/internal/game/scoring.go
tol  := math.Max(math.Abs(c) * 0.005, 1)   // ±0.5% or 1, whichever bigger
scale := math.Max(math.Abs(c) * 0.5,   10)  // half the correct, min 10
rankWeights := [3]float64{1.0, 0.66, 0.33}</pre>
      <p>
        Each guess is bucketed:
      </p>
      <ul>
        <li>
          Within <code>tol</code> of the correct value → <code>isCorrect=true</code>,
          <code>base + timeBonus</code>.
        </li>
        <li>
          Otherwise, sort by distance (ties broken by faster
          <code>responseMs</code>); the top three get
          <code>base × rankWeight × closeness</code>, where
          <code>closeness = max(0, 1 − diff/scale)</code>.
        </li>
        <li>Everyone else gets 0.</li>
      </ul>
      <p>
        Effect: a "perfect" answer (within the exact tolerance) gets the full
        base + time bonus, identical to a yes/no win. A wild guess that
        accidentally lands in the top 3 still earns very little — the
        closeness factor kills it.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Why decouple judging from persistence</h2>
      <p>
        <code>JudgeAnswer</code> and <code>ScoreNumberAnswers</code> are pure
        functions over <code>json.RawMessage</code>. No <code>*sql.Tx</code>,
        no <code>context.Context</code>, no HTTP. That makes
        <code>scoring_test.go</code> a tight table-driven test that runs in
        microseconds, and it makes the rules easy to argue about in isolation.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>No live re-scoring on number questions.</strong> Once
          revealed, the points stick. If the admin pulls the question back
          into "active" (currently not exposed), the rescore would need to
          run again.
        </li>
        <li>
          <strong>Time bonus is half the base, not a separate constant.</strong>
          A harder question rewards speed more in absolute terms. In a poll
          that compounds: the crowd's favourite answer is both worth more
          <em>and</em> earns a bigger bonus for arriving fast.
        </li>
        <li>
          <strong>Poll options are shuffled when written, not when read.</strong>
          Survey results arrive ranked, so storing them in that order would
          collapse the game into "always tap the top row". The shuffle happens
          once in <code>buildPollQuestion</code>; the admin editor re-sorts by
          points for display, so the host never sees the stored order.
        </li>
        <li>
          <strong>The number tolerance &amp; scale are absolute.</strong> Tuned
          for the "guess a year / weight / population" kind of question. A
          question whose correct value is 0 still works — the <code>min 1 / min 10</code>
          floors keep the math sane.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/ai" class="btn-link">Next: AI suggestions →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
