<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__sparkle s3" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase</span>
      <h1 class="hero__title">Implementation /<br /><em>tour</em></h1>
      <p class="hero__subtitle">
        How key parts of Trivia actually work. Annotated walkthroughs for
        contributors — not API reference.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <p>
        Each page below picks one slice of the system and explains the design
        choices behind it: which files matter, what shape the data takes, and
        why it was built that way. If you want a flat list of HTTP endpoints
        instead, see the
        <RouterLink to="/developers">API reference</RouterLink>.
      </p>
    </section>

    <section class="showcase-grid">
      <RouterLink class="showcase-card" to="/developers-showcase/auth">
        <span class="showcase-card__eyebrow">01 · Security</span>
        <h2 class="showcase-card__title">Authentication</h2>
        <p class="showcase-card__lead">
          Two credentials side-by-side: an HMAC admin JWT and opaque hex
          player tokens — plus the magic-link rejoin flow.
        </p>
        <span class="showcase-card__tag">JWT · tokens · email</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/images">
        <span class="showcase-card__eyebrow">02 · Media</span>
        <h2 class="showcase-card__title">Image upload &amp; handling</h2>
        <p class="showcase-card__lead">
          Content-addressed JPEGs in Postgres with pre-rendered variants,
          EXIF stripping, and an orphan-sweep GC.
        </p>
        <span class="showcase-card__tag">sha256 · variants · GC</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/database">
        <span class="showcase-card__eyebrow">03 · Storage</span>
        <h2 class="showcase-card__title">Database</h2>
        <p class="showcase-card__lead">
          Plain Postgres + pgx, file-based migrations, JSONB for answers, and
          the cascade graph that keeps deletes simple.
        </p>
        <span class="showcase-card__tag">pgx · JSONB · migrations</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/websocket">
        <span class="showcase-card__eyebrow">04 · Real-time</span>
        <h2 class="showcase-card__title">WebSocket &amp; live state</h2>
        <p class="showcase-card__lead">
          One hub, three roles, no fan-out bus. How players reconnect
          mid-question without losing their answer, and why the projector
          board needs no credential.
        </p>
        <span class="showcase-card__tag">hub · presence · ack replay</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/scoring">
        <span class="showcase-card__eyebrow">05 · Game</span>
        <h2 class="showcase-card__title">Scoring algorithm</h2>
        <p class="showcase-card__lead">
          Base points by difficulty, a linearly-decaying time bonus, the
          field-wide ranking behind number questions, and the poll format
          where every answer scores.
        </p>
        <span class="showcase-card__tag">points · time bonus · ranking</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/ai">
        <span class="showcase-card__eyebrow">06 · AI</span>
        <h2 class="showcase-card__title">AI question suggestions</h2>
        <p class="showcase-card__lead">
          One Anthropic Messages call, a strict-JSON system prompt, and a
          post-shuffle so the model can't tip off the answer.
        </p>
        <span class="showcase-card__tag">Claude · vision · prompt</span>
      </RouterLink>

      <RouterLink class="showcase-card" to="/developers-showcase/deployment">
        <span class="showcase-card__eyebrow">07 · Ops</span>
        <h2 class="showcase-card__title">Deployment &amp; Helm</h2>
        <p class="showcase-card__lead">
          A single Helm chart for frontend, backend, Postgres, and a Traefik 3
          Ingress — with sealed secrets and WebSocket upgrades out of the box.
        </p>
        <span class="showcase-card__tag">Helm · ingress · secrets</span>
      </RouterLink>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/" class="btn-link">← Back to home</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers" class="btn-link">API reference</RouterLink>
      <span aria-hidden="true">·</span>
      <a class="btn-link" href="https://github.com/oglimmer/trivia" target="_blank" rel="noopener noreferrer">Source on GitHub</a>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>

<style scoped>
.showcase-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 14px;
}

.showcase-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 18px 18px 16px;
  border: var(--bw) solid var(--ink);
  border-radius: var(--r);
  background: var(--paper);
  box-shadow: var(--shadow-1);
  text-decoration: none;
  color: var(--ink);
  transition: transform 120ms ease, box-shadow 120ms ease;
}
.showcase-card:hover {
  transform: translate(-2px, -2px);
  box-shadow: 6px 6px 0 var(--ink);
}
.showcase-card:nth-child(7n+1) { background: var(--mint-2); }
.showcase-card:nth-child(7n+2) { background: var(--yellow-2); }
.showcase-card:nth-child(7n+3) { background: var(--blue-2); }
.showcase-card:nth-child(7n+4) { background: var(--pink-2); }
.showcase-card:nth-child(7n+5) { background: var(--cream-2); }
.showcase-card:nth-child(7n+6) { background: var(--mint-2); }
.showcase-card:nth-child(7n+7) { background: var(--yellow-2); }

.showcase-card__eyebrow {
  font-family: var(--font-mono);
  font-size: .72rem;
  letter-spacing: .12em;
  text-transform: uppercase;
  font-weight: 700;
  color: var(--ink);
  opacity: .8;
}
.showcase-card__title {
  font-size: 1.25rem;
  line-height: 1.15;
  margin: 0;
}
.showcase-card__lead {
  margin: 0;
  font-size: .95rem;
  line-height: 1.45;
}
.showcase-card__tag {
  margin-top: auto;
  font-family: var(--font-mono);
  font-size: .72rem;
  letter-spacing: .04em;
  padding: 3px 8px;
  border-radius: 999px;
  border: 1.5px solid var(--ink);
  background: var(--paper);
  align-self: flex-start;
}
</style>
