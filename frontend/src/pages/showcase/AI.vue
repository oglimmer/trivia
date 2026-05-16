<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 06</span>
      <h1 class="hero__title">AI suggestions /<br /><em>one Claude call, strict JSON out</em></h1>
      <p class="hero__subtitle">
        Players can ask the backend to draft a trivia question from a photo
        and a hint. The model is constrained to a tight JSON shape and the
        choice order is shuffled before returning.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>One HTTP POST to Anthropic's <code>/v1/messages</code> endpoint.</li>
        <li>The optional photo rides as a base64 image block; the canonical <code>medium</code> variant is used to keep payload small.</li>
        <li>System prompt enforces strict-JSON output and quality rules.</li>
        <li>The parser is permissive — it extracts the first <code>{…}</code> block so the model can wrap output in prose.</li>
        <li>For <code>choice</code> answers the option order is shuffled <em>and</em> the correct index rewritten, so the model's bias toward option <code>0</code> doesn't tip off the player.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/ai/claude.go</code> — client, system prompt, parser, shuffler.</li>
        <li><code>backend/internal/ai/claude_test.go</code> — exercised against an <code>httptest</code> double.</li>
        <li><code>backend/internal/api/player.go</code> — <code>aiSuggest</code> handler bridges HTTP to the AI client.</li>
        <li><code>helm/trivia/values.yaml</code> — <code>anthropic.model</code>; API key in sealed secret.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Construction &amp; configuration</h2>
      <pre class="api-code">// backend/internal/ai/claude.go
func New() *Client {
    model := os.Getenv("ANTHROPIC_MODEL")
    if model == "" {
        model = "claude-sonnet-4-6"
    }
    return &amp;Client{
        APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
        Model:   model,
        BaseURL: defaultBaseURL,
        HTTP:    &amp;http.Client{Timeout: 30 * time.Second},
    }
}</pre>
      <p>
        <code>BaseURL</code> is overridable so the unit tests can point at an
        <code>httptest.NewServer</code> — no network calls in CI. The 30 s
        client timeout is wider than a single message turn typically needs but
        keeps a slow upstream from holding the player's HTTP request open
        indefinitely. Failures bubble up as HTTP 502 to the SPA.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The request</h2>
      <p>
        Two content blocks per turn: the image (optional) and the prompt
        text. The image is base64'd into a <code>type: image</code> block:
      </p>
      <pre class="api-code">userBlocks = append(userBlocks, map[string]any{
    "type": "image",
    "source": map[string]any{
        "type":       "base64",
        "media_type": mediaType, // "image/jpeg" by default
        "data":       base64.StdEncoding.EncodeToString(image.Data),
    },
})
userBlocks = append(userBlocks, map[string]any{
    "type": "text",
    "text": fmt.Sprintf(
        "answerType=%s\nhint=%s\nWrite the question now.",
        req.AnswerType, req.Hint,
    ),
})</pre>
      <p>
        On the API side the backend resolves <code>photoImageId</code> to the
        <code>medium</code> variant before calling the AI client. That keeps
        the request payload small (≤ 640 px max edge) without the model
        losing context.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The system prompt</h2>
      <p>
        Stored as a single Go string literal next to the client — easier to
        diff and review than loading from disk:
      </p>
      <pre class="api-code">var systemPrompt = `You write ONE genuinely surprising trivia question ...

Return a strict JSON object with these fields and nothing else (no prose, no code fences):
{ "text": string, "options": string[], "correct": any }

Rules by answerType:
- yesno    : options=["Yes","No"]; correct="yes" or "no".
- choice   : options=exactly 4 short answers; correct=integer index (0-based).
- number   : options=[]; correct is the numeric answer (no units).
...`</pre>
      <p>
        The prompt does two jobs at once: the <em>shape</em> rules
        (strict JSON, per-answer-type structure) and the <em>quality</em>
        rules (a counter-intuitive fact, plausible-but-wrong distractors,
        no leading the witness with "did you know"). It's authoritative — no
        retry/repair loop in code.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Parsing</h2>
      <p>
        The model is told to return JSON, but reality intervenes. The parser
        slices out the first <code>{</code>...<code>}</code> block before
        decoding so a code fence or stray prose doesn't fail the whole call:
      </p>
      <pre class="api-code">text := ar.Content[0].Text
start := strings.IndexByte(text, '{')
end   := strings.LastIndexByte(text, '}')
if start &lt; 0 || end &lt;= start {
    return nil, fmt.Errorf("could not parse JSON from: %s", text)
}
out := &amp;SuggestResponse{}
if err := json.Unmarshal([]byte(text[start:end+1]), out); err != nil {
    return nil, fmt.Errorf("invalid JSON: %w; raw=%s", err, text)
}</pre>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Why shuffle the options</h2>
      <p>
        Language models put the correct choice first more often than chance
        would predict. If the SPA rendered the options in the order the model
        returned them, a savvy player would have a tell. So the client
        shuffles in place and rewrites the correct index to match:
      </p>
      <pre class="api-code">func shuffleChoices(r *SuggestResponse) {
    n := len(r.Options)
    if n &lt; 2 { return }
    correctIdx, ok := correctAsIndex(r.Correct, n)
    if !ok { return }
    perm := rand.Perm(n)
    shuffled := make([]string, n)
    newCorrect := 0
    for newPos, oldPos := range perm {
        shuffled[newPos] = r.Options[oldPos]
        if oldPos == correctIdx {
            newCorrect = newPos
        }
    }
    r.Options = shuffled
    r.Correct = float64(newCorrect) // keep the JSON-encoded type (float64)
}</pre>
      <p>
        Only <code>choice</code> answers are shuffled — yes/no and number
        don't have position bias to worry about.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>No streaming.</strong> The handler waits for the full
          response. Question drafts are short (≤ 512 max tokens); streaming
          would add UX complexity for marginal latency.
        </li>
        <li>
          <strong>No retry on bad JSON.</strong> If the model returns
          something we can't parse, the call fails and the SPA shows a "try
          again" affordance to the user. That's cheap and keeps the code path
          obvious.
        </li>
        <li>
          <strong>Model is configurable but defaults sensibly.</strong>
          <code>ANTHROPIC_MODEL</code> overrides the default
          (<code>claude-sonnet-4-6</code>). The Helm chart surfaces this as
          <code>anthropic.model</code>.
        </li>
        <li>
          <strong>API key in sealed secret.</strong>
          <code>ANTHROPIC_API_KEY</code> is never written to the values file
          — see the <RouterLink to="/developers-showcase/deployment">deployment showcase</RouterLink>.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/deployment" class="btn-link">Next: Deployment →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
