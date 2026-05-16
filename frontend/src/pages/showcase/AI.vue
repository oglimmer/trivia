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
        <li>The <code>web_search_20260209</code> server tool is enabled (capped at 3 uses) so the model can verify obscure facts before committing them — and the API auto-injects code-execution server-side for dynamic result filtering.</li>
        <li>The parser walks content blocks in reverse to find the last text block containing JSON, since tool use produces interleaved <code>server_tool_use</code> / <code>web_search_tool_result</code> blocks; within that block it extracts the first <code>{…}</code> so wrapping prose is tolerated.</li>
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
        // 30s was tight enough when this was a plain one-shot completion.
        // With web_search + dynamic filtering the request can fan out to
        // 3 searches plus code-execution filtering plus vision reasoning,
        // and 60–90s end-to-end is normal. 120s is a comfortable ceiling.
        HTTP: &amp;http.Client{Timeout: 120 * time.Second},
    }
}</pre>
      <p>
        <code>BaseURL</code> is overridable so the unit tests can point at an
        <code>httptest.NewServer</code> — no network calls in CI. The 120 s
        client timeout looks generous but matches the real upper bound of a
        web-search-assisted turn; the Setup modal sets player expectations
        with a "lowkey just wait 90 seconds" copy. Failures bubble up as HTTP
        502 to the SPA.
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
      <p>
        The request body also declares the <code>web_search</code> server
        tool so the model can fact-check before committing to a number, year,
        or name:
      </p>
      <pre class="api-code">// web_search_20260209 enables dynamic filtering (Claude writes code
// to post-filter search results before they hit context). The API
// auto-injects the required code_execution tool server-side;
// declaring it ourselves causes a 400 "tool names must be unique".
var webSearchTool = map[string]any{
    "type":     "web_search_20260209",
    "name":     "web_search",
    "max_uses": 3, // cost ceiling: $10 per 1k searches
}

body := anthropicReq{
    Model:     c.Model,
    MaxTokens: 4096, // bumped from 512 — tool turns + JSON answer
    System:    systemPrompt,
    Messages: []map[string]any{
        {"role": "user", "content": userBlocks},
    },
    Tools: []map[string]any{webSearchTool},
}</pre>
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
...

Use the web_search tool to verify the fact before committing to it. Specifically:
- If you are not &gt;95% confident the fact is true AND that the specific
  value (number, year, name) is correct, search to confirm.
- For "number" and "choice" questions, search to pin down the exact figure
  rather than guessing — a question with a wrong "correct" answer is worse
  than a boring one.
- Prefer facts you can corroborate from a reputable source. If search
  contradicts your initial idea, change the fact rather than the source.

Hard rules:
- ...
- Your FINAL message must be the JSON object alone — no prose around it,
  no code fences. Any reasoning or search results stay in the tool-use turns.`</pre>
      <p>
        The prompt does three jobs at once: the <em>shape</em> rules (strict
        JSON, per-answer-type structure), the <em>quality</em> rules (a
        counter-intuitive fact, plausible-but-wrong distractors, no leading
        the witness with "did you know"), and the <em>tool policy</em> (when
        to call <code>web_search</code> and where the final JSON has to land).
        It's authoritative — no retry/repair loop in code.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Parsing</h2>
      <p>
        The model is told to return JSON, but two things complicate
        extraction. First, with web search enabled the <code>content</code>
        array is now a mix of block types — early <code>text</code> blocks
        may be the model narrating its plan ("let me verify that with a
        quick search"), then <code>server_tool_use</code> and
        <code>web_search_tool_result</code> blocks, and finally the answer
        text. We walk content in reverse and pick the last text block that
        contains a <code>{</code>:
      </p>
      <pre class="api-code">// With the web_search tool enabled, content can be a mix of text,
// server_tool_use, and web_search_tool_result blocks. The JSON answer
// is in the last text block — earlier text may be the model narrating
// its search plan before the tool fires.
text := ""
for i := len(ar.Content) - 1; i &gt;= 0; i-- {
    if ar.Content[i].Type == "text" &amp;&amp; strings.ContainsRune(ar.Content[i].Text, '{') {
        text = ar.Content[i].Text
        break
    }
}
if text == "" {
    return nil, fmt.Errorf("no text block with JSON in response: %s", string(rb))
}</pre>
      <p>
        Second, inside that text block the parser still slices out the first
        <code>{</code>...<code>}</code> region before decoding so a code
        fence or stray prose doesn't fail the whole call:
      </p>
      <pre class="api-code">start := strings.IndexByte(text, '{')
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
          response. <code>max_tokens</code> is 4096 (raised from 512 to leave
          headroom for tool turns plus the answer JSON); a busy modal with a
          spinner is enough UX for the 60–90 s wait.
        </li>
        <li>
          <strong>Web search is capped at 3 uses.</strong> Most prompts only
          need 1–2 searches; the cap is a soft cost ceiling
          ($10 per 1k searches). Anthropic auto-injects the
          <code>code_execution</code> tool server-side to enable dynamic
          filtering — declaring it ourselves alongside <code>web_search</code>
          returns a 400 "tool names must be unique".
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
