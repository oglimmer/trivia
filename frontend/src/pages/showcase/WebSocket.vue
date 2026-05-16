<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 04</span>
      <h1 class="hero__title">Real-time /<br /><em>one hub, six envelopes</em></h1>
      <p class="hero__subtitle">
        Live game state is pushed over one WebSocket endpoint, scoped by game.
        Players and admins share the same wire — the role is set at handshake
        time.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>One process, one in-memory <code>Hub</code>, no Redis fan-out — single-replica by design.</li>
        <li>Connections live in "rooms" keyed by <code>gameID</code>.</li>
        <li>The hub doesn't know anything about questions or scoring — domain logic lives in <code>internal/api</code>, hooked via three callbacks: <code>OnJoin</code> / <code>OnLeave</code> / <code>OnRecv</code>.</li>
        <li>Reconnects replay an <code>answerAck</code> if the player already answered the active question, so refresh-mid-question lands on the right view.</li>
        <li>30 s server ping / 75 s read deadline keep dead sockets from piling up.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/ws/hub.go</code> — Hub, Client, read/write loops, broadcast helpers.</li>
        <li><code>backend/internal/api/ws.go</code> — handshake, role selection, message dispatch.</li>
        <li><code>backend/internal/api/broadcast.go</code> — envelope shapes (gameState, users, presence, …).</li>
        <li><code>frontend/src/composables/useWebSocket.ts</code> — client-side reconnect loop.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Handshake &amp; role</h2>
      <p>
        Both roles use the same endpoint; the query string picks which:
      </p>
      <pre class="api-code">// Player
GET /ws?token=&lt;playerToken&gt;

// Admin
GET /ws?role=admin&amp;token=&lt;adminJWT&gt;&amp;code=&lt;gameCode&gt;</pre>
      <p>
        <strong>Why query parameters instead of a header?</strong> Browser
        <code>WebSocket</code> constructors don't let you set arbitrary
        request headers. Putting the token in the URL is the only portable
        option — and it's the same secret either way, so it doesn't widen the
        threat model.
      </p>
      <pre class="api-code">// backend/internal/api/ws.go
if r.URL.Query().Get("role") == "admin" {
    c, err := auth.Parse(tok)
    if err != nil || c.Role != "admin" { /* 401 */ }
    g, _ := s.DB.GameByCode(r.Context(), code)
    role = ws.RoleAdmin
    gameID = g.ID
} else {
    u, err := s.DB.UserByToken(r.Context(), tok)
    if err != nil { /* 401 */ }
    gameID = u.GameID
    userID = u.ID
}
s.Hub.Serve(w, r, gameID, userID, role)</pre>
      <p>
        From this point on, the hub doesn't care which credential type was
        used — <code>Role</code> + <code>UserID</code> are enough for every
        downstream check.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The hub</h2>
      <p>
        <code>Hub</code> is a small, hub-and-spoke goroutine coordinator:
      </p>
      <pre class="api-code">// backend/internal/ws/hub.go
type Hub struct {
    mu      sync.RWMutex
    rooms   map[string]map[*Client]struct{} // gameID -&gt; clients
    OnRecv  func(c *Client, msg []byte)
    OnJoin  func(c *Client)
    OnLeave func(c *Client)
}</pre>
      <p>
        Each <code>Client</code> owns two goroutines:
      </p>
      <ul>
        <li><code>readLoop</code> — blocks on <code>conn.ReadMessage()</code>, fires <code>OnRecv</code>.</li>
        <li><code>writeLoop</code> — drains a buffered <code>send chan []byte</code>, fires a 30 s ping ticker.</li>
      </ul>
      <p>
        Slow consumers get dropped, not blocked. The send channel is buffered
        to 256; if it fills, the broadcaster takes the <code>default</code>
        branch and drops the message for that client:
      </p>
      <pre class="api-code">for c := range room {
    select {
    case c.send &lt;- b:
    default:
        // slow consumer; drop
    }
}</pre>
      <p>
        This is a deliberate trade — better to drop a <code>users</code> push
        for one client than to wedge every other client behind it.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Wire envelopes</h2>
      <pre class="api-code">// Outbound shapes — see internal/api/broadcast.go
{ "type": "gameState",    "data": { /* full game view */ } }
{ "type": "users",        "data": User[]   }
{ "type": "questionsAdmin","data": Question[] } // admin only
{ "type": "presence",     "data": { online: string[] } } // admin only
{ "type": "answerAck",    "data": { questionId, responseMs } }
{ "type": "playerAnswered","data": { userId, questionId, responseMs } } // admin only
{ "type": "gameDeleted" }</pre>
      <p>
        Inbound (client → server) is just <code>answer</code> and
        <code>ping</code>:
      </p>
      <pre class="api-code">{ "type": "answer", "data": { "questionId": "...", "value": ... } }
{ "type": "ping" }</pre>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Reconnect replay</h2>
      <p>
        The single most player-visible piece of design: if you've already
        locked in your answer and refresh the page, you should land on
        "Locked in!" — not on the answer buttons. The hub doesn't know
        anything about that; the API's <code>OnJoin</code> handler does:
      </p>
      <pre class="api-code">// backend/internal/api/ws.go
if c.Role == ws.RolePlayer &amp;&amp; g.QuestionState == "active" &amp;&amp; g.CurrentQuestionID != nil {
    if ans, _ := s.DB.AnswersForQuestion(ctx, *g.CurrentQuestionID); ans != nil {
        for _, a := range ans {
            if a.UserID == c.UserID {
                c.Send(map[string]any{
                    "type": "answerAck",
                    "data": map[string]any{
                        "questionId": a.QuestionID,
                        "responseMs": a.ResponseMs,
                    },
                })
                break
            }
        }
    }
}
c.Send(s.gameStateEnvelope(ctx, g, c.Role == ws.RoleAdmin))</pre>
      <p>
        Order matters: the <code>answerAck</code> goes <em>before</em>
        <code>gameState</code>, so the client transitions to "locked in" the
        moment the state lands.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Presence</h2>
      <p>
        "Who is online right now?" is purely derived from live connections —
        it isn't stored. <code>OnlinePlayers</code> walks the room once and
        de-dups by <code>userId</code>:
      </p>
      <pre class="api-code">func (h *Hub) OnlinePlayers(gameID string) []string {
    h.mu.RLock(); defer h.mu.RUnlock()
    seen := make(map[string]struct{})
    for c := range h.rooms[gameID] {
        if c.Role == RolePlayer &amp;&amp; c.UserID != "" {
            seen[c.UserID] = struct{}{}
        }
    }
    out := make([]string, 0, len(seen))
    for id := range seen { out = append(out, id) }
    return out
}</pre>
      <p>
        A player can have multiple tabs open — the de-dup means presence is
        per-user, not per-socket.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Late answers &amp; auto-close</h2>
      <p>
        The answer handler computes response time from
        <code>question_started_at</code> and rejects anything past the
        per-question timeout. This is what protects against a small race
        between the auto-close timer firing and the client sending a final
        answer:
      </p>
      <pre class="api-code">// backend/internal/api/ws.go
responseMs := int(time.Since(*g.QuestionStartedAt) / time.Millisecond)
if g.QuestionTimeoutSeconds &gt; 0 &amp;&amp; responseMs &gt; g.QuestionTimeoutSeconds*1000 {
    return // silently drop
}</pre>
      <p>
        Server-side auto-close lives in <code>api/server.go</code> as a
        per-game <code>time.AfterFunc</code>. On startup,
        <code>ResumeAutoCloseTimers</code> walks every game with an active
        question and re-arms its timer — so a backend restart can't strand
        players on an expired question.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>Origin check is permissive.</strong>
          <code>CheckOrigin: func(*http.Request) bool { return true }</code>.
          The credential check happens on the next line — origin alone isn't
          load-bearing. If the deployment ever serves untrusted browser
          extensions, tighten this.
        </li>
        <li>
          <strong>Single-replica.</strong> The hub is process-local. Adding a
          second backend replica without a fan-out bus would silently split
          players across rooms.
        </li>
        <li>
          <strong>30 s ping, 75 s read deadline.</strong> Three missed pings
          before a peer is considered dead — keeps idle tabs cheap without
          being trigger-happy on a flaky network.
        </li>
        <li>
          <strong>The ingress lets WebSockets through.</strong> Traefik 3
          handles the <code>Upgrade</code> handshake transparently with no
          per-Ingress annotations — covered in the
          <RouterLink to="/developers-showcase/deployment">deployment showcase</RouterLink>.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/scoring" class="btn-link">Next: Scoring →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
