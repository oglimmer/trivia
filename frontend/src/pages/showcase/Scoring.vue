<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 05</span>
      <h1 class="hero__title">Scoring /<br /><em>fast first, close enough later</em></h1>
      <p class="hero__subtitle">
        Yes/no and choice answers score the moment they arrive. Number
        answers score at reveal time, ranked against the whole field.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>Base points scale with the difficulty floor of the answer type.</li>
        <li>A linearly-decaying time bonus rewards fast correct answers.</li>
        <li>Yes/no &amp; choice → graded individually as they arrive (<code>JudgeAnswer</code>).</li>
        <li>Number → graded as a field at reveal time (<code>ScoreNumberAnswers</code>).</li>
        <li>All scoring lives in one file with one dependency: the Go standard library.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/game/scoring.go</code> — every scoring rule.</li>
        <li><code>backend/internal/game/scoring_test.go</code> — table-driven coverage.</li>
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
//   choice with 4 options  300
//   number                 300 (open-ended)</pre>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Time bonus</h2>
      <p>
        Correct yes/no and choice answers get an extra <code>0..base/2</code>
        that decays linearly to 0 at the answer window (30 s).
      </p>
      <pre class="api-code">const AnswerWindowMs = 30_000

func timeBonus(base, responseMs int) int {
    if responseMs &lt; 0           { responseMs = 0 }
    if responseMs &gt;= AnswerWindowMs { return 0 }
    frac := 1.0 - float64(responseMs)/float64(AnswerWindowMs)
    return int(math.Round(float64(base) * 0.5 * frac))
}</pre>
      <p>
        So a 2-option choice answered at 0 ms is worth <code>100 + 50 = 150</code>.
        Answered at 15 s, <code>100 + 25 = 125</code>. Answered at 30 s+,
        <code>100</code>. Answered <em>after</em> the per-question timeout, 0 —
        the answer is rejected at the WebSocket layer before scoring even runs.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Yes/no &amp; choice (immediate)</h2>
      <p>
        Both run through <code>JudgeAnswer</code> in the same WebSocket message
        handler that persists the answer.
      </p>
      <pre class="api-code">func JudgeAnswer(answerType string, optionCount int,
                 correct, answer json.RawMessage,
                 responseMs int) (isCorrect bool, points int) {
    base := basePoints(answerType, optionCount)
    switch answerType {
    case "yesno":
        var c, a string
        _ = json.Unmarshal(correct, &amp;c)
        _ = json.Unmarshal(answer, &amp;a)
        if normYesNo(c) == normYesNo(a) {
            return true, base + timeBonus(base, responseMs)
        }
    case "choice":
        var c, a int
        if json.Unmarshal(correct, &amp;c) != nil { return false, 0 }
        if json.Unmarshal(answer,  &amp;a) != nil { return false, 0 }
        if c == a {
            return true, base + timeBonus(base, responseMs)
        }
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
    for _, sc := range game.ScoreNumberAnswers(q.Correct, inputs) {
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
          A harder question rewards speed more in absolute terms.
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
