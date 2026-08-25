<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 01</span>
      <h1 class="hero__title">Authentication /<br /><em>two credentials, one surface</em></h1>
      <p class="hero__subtitle">
        Admins use an HMAC JWT; players use an opaque hex token. Plus the
        magic-link rejoin email that gets a player back into a game from any
        device — and one view that deliberately carries no credential at all.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <p>
        The backend deliberately runs two parallel credential schemes — one
        designed around a single trusted operator, one designed around many
        short-lived guests. They never mix: an admin JWT cannot answer a
        question, and a player token cannot open the admin panel.
      </p>
      <ul>
        <li>
          <strong>Admin</strong> — single shared password →
          <code>HS256</code>-signed JWT, 24-hour TTL.
        </li>
        <li>
          <strong>Player</strong> — 16-byte random hex string minted on
          <code>POST /api/games/{code}/join</code>. No expiry; stored on the
          user row.
        </li>
        <li>
          <strong>Magic link</strong> — same player token, delivered by email,
          consumed by the <code>/impersonate</code> SPA route.
        </li>
        <li>
          <strong>Board</strong> — the projector view carries
          <em>no</em> credential. See below.
        </li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/auth/jwt.go</code> — issue/parse, <code>RequireAdmin</code> middleware.</li>
        <li><code>backend/internal/api/admin.go</code> — <code>adminLogin</code> handler.</li>
        <li><code>backend/internal/api/helpers.go</code> — <code>playerFromHeader</code>, <code>randomToken</code>.</li>
        <li><code>backend/internal/api/player.go</code> — <code>joinGame</code>, <code>sendLoginLink</code>.</li>
        <li><code>backend/internal/mail/mail.go</code> — magic-link email + <code>LoginLinkURL</code>.</li>
        <li><code>frontend/src/pages/Impersonate.vue</code> — consumes the magic-link hash.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Admin JWT</h2>
      <p>
        The admin "login" is a constant-time compare against the
        <code>ADMIN_PASSWORD</code> environment variable. If it matches, the
        handler mints a JWT with two claims that matter:
        <code>sub=admin</code> and <code>role=admin</code>.
      </p>
      <pre class="api-code">// backend/internal/auth/jwt.go
func Issue(sub, role string, ttl time.Duration) (string, error) {
    c := Claims{
        Subject: sub,
        Role:    role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
    return t.SignedString(secret())
}</pre>
      <p>
        Verification refuses any signing method that isn't HMAC. This
        defends against the classic "<code>alg: none</code>" downgrade — the
        callback only returns the secret if the token actually used HS256.
      </p>
      <pre class="api-code">parsed, err := jwt.ParseWithClaims(tok, &amp;Claims{}, func(t *jwt.Token) (any, error) {
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, errors.New("bad signing method")
    }
    return secret(), nil
})</pre>
      <p>
        <code>RequireAdmin</code> is the gate every admin-only HTTP route
        flows through. The mounting is in <code>routes.go</code>:
      </p>
      <pre class="api-code">r.Group(func(r chi.Router) {
    r.Use(auth.RequireAdmin)
    r.Get("/admin/games", s.listGames)
    // ... every other admin route
})</pre>
      <p>
        The WebSocket handshake re-uses the same parser — see
        <RouterLink to="/developers-showcase/websocket">WebSocket showcase</RouterLink>
        for why the token rides in a query parameter there instead of a header.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Player tokens</h2>
      <p>
        A player token is just <code>16 bytes of crypto/rand, hex-encoded</code>
        — 128 bits of entropy. No JWT, no DB-side cryptography, no rotation;
        the token <em>is</em> the user identifier the server keys on.
      </p>
      <pre class="api-code">// backend/internal/api/helpers.go
func randomToken(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}</pre>
      <p>
        Players send it on every HTTP call as <code>X-Player-Token</code>.
        The helper that resolves the header also touches <code>last_seen</code>,
        which is what powers the "prune idle players when the game starts" sweep.
      </p>
      <pre class="api-code">// backend/internal/api/helpers.go
func (s *Server) playerFromHeader(r *http.Request) (*db.User, error) {
    tok := r.Header.Get("X-Player-Token")
    if tok == "" {
        return nil, errors.New("missing player token")
    }
    u, err := s.DB.UserByToken(r.Context(), tok)
    if err != nil {
        return nil, err
    }
    if err := s.DB.TouchUserLastSeen(r.Context(), u.ID); err != nil {
        log.Printf("touch last_seen for %s: %v", u.ID, err)
    }
    return u, nil
}</pre>
      <p>
        <strong>Why not JWT for players?</strong> A trivia round has dozens of
        ephemeral users that disappear when the game ends. There is no
        cross-tenant authorization to encode, no claim worth signing — just
        "are you the human that joined as Ada?". A 16-byte random string is the
        smallest credential that answers that question.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Magic-link rejoin</h2>
      <p>
        If a player supplies an email on join (or via <code>PUT /api/me</code>),
        the server queues a one-click sign-in email containing the same player
        token. There are no separate "login link" tokens to mint, store, or
        expire — the magic link is just the existing capability rendered as a URL.
      </p>
      <pre class="api-code">// backend/internal/mail/mail.go
func (m *Mailer) LoginLinkURL(playerToken string) string {
    if m.PublicBaseURL == "" {
        return "/impersonate#token=" + playerToken
    }
    return m.PublicBaseURL + "/impersonate#token=" + playerToken
}</pre>
      <p>
        The token rides in the URL <strong>fragment</strong>, not the query.
        Fragments are never sent to servers, so they don't leak into
        access logs, referrer headers, or the Postgres slow-query log. The
        <code>Impersonate.vue</code> page reads <code>location.hash</code>,
        stows the token in localStorage, then replaces the URL.
      </p>
      <p>
        Send is fire-and-forget — a flaky SMTP server should not break the
        join HTTP response:
      </p>
      <pre class="api-code">// backend/internal/api/player.go
func (s *Server) sendLoginLink(email, playerName, gameName, gameCode, token string) {
    if s.Mail == nil || email == "" {
        return
    }
    go func() {
        if err := s.Mail.SendLoginLink(email, playerName, gameName, gameCode, token); err != nil {
            log.Printf("send login link to %q: %v", email, err)
        }
    }()
}</pre>
      <p>
        When SMTP is disabled (the default in dev), the mailer logs the URL to
        stdout — so manual rejoin flows still work without standing up a relay.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The third case: no credential at all</h2>
      <p>
        The projector board at <code>/g/{code}/board</code> is the one surface
        that authenticates nothing. It opens a WebSocket as
        <code>?role=board&amp;code=&lt;gameCode&gt;</code>, and anyone who knows
        the game code can open it.
      </p>
      <p>
        The reasoning is about what a credential would protect. A board is a TV
        in a room where the game is already being played out loud — everything
        on it is visible to everyone present by design. Issuing it a token
        would mean storing a secret on a machine anybody in the room can walk
        up to, in exchange for guarding information the room already has.
      </p>
      <p>
        What it does <em>not</em> get is the admin view. The board is served the
        player-facing envelope, so Company Consensus option points stay hidden
        until the host reveals them. It also carries an empty
        <code>UserID</code>, which is what keeps it out of presence counts and
        makes the answer handler ignore anything it sends:
      </p>
      <pre class="api-code">// backend/internal/api/ws.go
if c.Role != ws.RolePlayer {
    return // only players can answer
}</pre>
      <p>
        So the credential model is still two schemes, plus one role defined
        entirely by what it cannot do. Read-only by construction beats
        read-only by permission check.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>Constant-time compare</strong> on the admin password:
          <code>subtle.ConstantTimeCompare</code>, not <code>==</code>. Trivial
          here, habitual everywhere.
        </li>
        <li>
          <strong>No refresh tokens.</strong> 24-hour admin TTL is short
          enough that re-login is fine; player tokens just don't expire.
        </li>
        <li>
          <strong>The JWT secret defaults to a dev placeholder.</strong> The
          deployment chart wires <code>JWT_SECRET</code> from a sealed
          secret — <em>do not</em> ship the default.
        </li>
        <li>
          <strong>Name uniqueness</strong> is enforced at the DB level via a
          case-insensitive partial index (migration <code>0009</code>); the
          handler maps that constraint to HTTP <code>409</code>.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/images" class="btn-link">Next: Images →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
