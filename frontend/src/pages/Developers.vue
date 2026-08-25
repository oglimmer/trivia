<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__sparkle s3" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Reference</span>
      <h1 class="hero__title">Developers /<br /><em>API docs</em></h1>
      <p class="hero__subtitle">The HTTP and WebSocket surface that powers Trivia.</p>
      <p class="hero__subtitle" style="margin-top: 8px;">
        Looking for <em>how things actually work</em> instead of an endpoint
        list? See the
        <RouterLink to="/developers-showcase">implementation showcase</RouterLink>.
      </p>
    </section>

    <!-- ---------- Overview ---------- -->
    <section class="card stack legal-prose api-prose">
      <p>
        Trivia exposes a small JSON HTTP API plus a WebSocket channel for
        real-time game state. All HTTP endpoints live under <code>/api</code>
        and accept and return <code>application/json</code> unless otherwise
        noted. Responses use standard HTTP status codes; error bodies look like
        <code>{"error": "message"}</code>.
      </p>

      <h2>Game modes</h2>
      <p>
        A game is created in one of two modes, and the mode decides who writes
        the questions and how they score. It is fixed at creation and reported
        as <code>mode</code> on every <code>Game</code> payload.
      </p>
      <ul>
        <li>
          <code>classic</code> — each player writes one question with a photo
          during <code>setup</code>. Every question has one correct answer.
          This is the default when <code>mode</code> is omitted.
        </li>
        <li>
          <code>poll</code> — Company Consensus. The host authors the whole
          set up front from survey results, players join as teams, and every
          option scores by how many survey respondents gave it. Question
          order is played as uploaded rather than shuffled.
        </li>
      </ul>
      <p>
        Endpoints that only apply to one mode say so. Calling a
        <code>poll</code>-only endpoint on a classic game returns
        <code>400</code>, and vice versa.
      </p>

      <h2>Authentication</h2>
      <p>The API has two independent credential types:</p>
      <ul>
        <li>
          <strong>Admin JWT</strong> — issued by
          <code>POST /api/admin/login</code>, valid for 24 hours. Send it as
          <code>Authorization: Bearer &lt;token&gt;</code> on admin routes, or
          as the <code>token</code> query parameter when opening the admin
          WebSocket.
        </li>
        <li>
          <strong>Player token</strong> — opaque hex string returned by
          <code>POST /api/games/{code}/join</code> (or via the magic-link
          email). Send it as the <code>X-Player-Token</code> header on player
          routes, or as the <code>token</code> query parameter when opening
          the player WebSocket.
        </li>
      </ul>
      <p>
        Endpoints under <code>/api/images</code> and the public game lookups
        are unauthenticated — possession of the image UUID or the
        game code is the capability.
      </p>
    </section>

    <!-- ---------- General ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>General</h2>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/health</code>
        </header>
        <p>Liveness probe. Returns <code>200 OK</code> with body <code>ok</code>. No auth.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/version</code>
        </header>
        <p>Build metadata for the running backend.</p>
        <pre class="api-code">{
  "name": "trivia-backend",
  "version": "v1.2.3",
  "gitCommit": "abc1234",
  "buildTime": "2026-05-15T10:21:00Z"
}</pre>
      </article>
    </section>

    <!-- ---------- Admin ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>Admin</h2>
      <p>
        All endpoints below require
        <code>Authorization: Bearer &lt;adminJWT&gt;</code> except
        <code>POST /api/admin/login</code>.
      </p>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/login</code>
        </header>
        <p>Exchange the admin password for a JWT.</p>
        <p><strong>Request:</strong></p>
        <pre class="api-code">{ "password": "letmein" }</pre>
        <p><strong>200 response:</strong></p>
        <pre class="api-code">{ "token": "eyJhbGciOi..." }</pre>
        <p>Returns <code>401</code> on the wrong password.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/admin/games</code>
        </header>
        <p>List every game with live online-player counts. Returns an array of game summaries.</p>
        <pre class="api-code">[
  {
    "id": "uuid",
    "code": "abcd",
    "name": "Game night",
    "mode": "classic",
    "state": "setup",
    "hideLeaderboardTail": true,
    "currentQuestionId": null,
    "questionState": "idle",
    "questionStartedAt": null,
    "questionClosedAt": null,
    "questionTimeoutSeconds": 30,
    "scheduledAt": null,
    "createdAt": "2026-05-15T18:00:00Z",
    "onlineCount": 4
  }
]</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games</code>
        </header>
        <p>Create a new game. Code is generated if omitted. Timeout is clamped to <code>[5, 600]</code> seconds; <code>0</code>/missing defaults to <code>30</code>.</p>
        <pre class="api-code">{
  "code": "abcd",                       // optional
  "name": "Game night",
  "questionTimeoutSeconds": 30,         // optional
  "scheduledAt": "2026-05-20T19:00:00Z", // optional, RFC3339
  "mode": "classic" | "poll"            // optional, defaults to "classic"
}</pre>
        <p>
          Returns the created <code>Game</code> object. Any other
          <code>mode</code> value is rejected with <code>400</code>. A
          <code>poll</code> game is created with
          <code>hideLeaderboardTail</code> off so the TV board can show
          standings all the way through; classic games keep it on.
        </p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/admin/users</code>
        </header>
        <p>Returns every user across all games (admin overview).</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/admin/games/{code}</code>
        </header>
        <p>Full snapshot of one game.</p>
        <pre class="api-code">{
  "game":      Game,
  "users":     User[],
  "questions": Question[],   // includes "correct"
  "online":    string[]      // userIds currently connected
}</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--delete">DELETE</span>
          <code class="api-path">/api/admin/games/{code}</code>
        </header>
        <p>Delete a game. Cascades to users, questions, answers; sweeps newly-orphaned images.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/state</code>
        </header>
        <p>Move the game between lifecycle phases.</p>
        <pre class="api-code">{ "state": "setup" | "game" | "finished" }</pre>
        <p>
          Transitioning to <code>game</code> prunes players idle &gt; 30
          minutes, shuffles question order, and clears the active question.
          Responds <code>204 No Content</code>.
        </p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--put">PUT</span>
          <code class="api-path">/api/admin/games/{code}/settings</code>
        </header>
        <p>Editable only in <code>setup</code>. Every field is optional; <code>scheduledAt</code> may be sent as explicit <code>null</code> to clear.</p>
        <pre class="api-code">{
  "questionTimeoutSeconds": 45,
  "scheduledAt": "2026-05-20T19:00:00Z",
  "hideLeaderboardTail": true
}</pre>
        <p>
          <code>hideLeaderboardTail</code> hides everything below the top few
          places until the final reveal. It is on by default, and off for
          <code>poll</code> games, where a live board is the point.
        </p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/activate</code>
        </header>
        <p>Open a specific question for answers. If <code>questionId</code> is omitted, the next question in sort order is picked.</p>
        <pre class="api-code">{ "questionId": "uuid" }     // optional</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/reveal</code>
        </header>
        <p>Close the active question and reveal the correct answer. Triggers final scoring for number questions.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/next</code>
        </header>
        <p>Advance to the next question, or finish the game if there are no more.</p>
        <pre class="api-code">// More questions:
{ "done": false, "questionId": "uuid" }

// No more — game flipped to "finished":
{ "done": true }</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/finish</code>
        </header>
        <p>Force-finish the game.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--delete">DELETE</span>
          <code class="api-path">/api/admin/games/{code}/users/{userId}</code>
        </header>
        <p>Remove a player. Only allowed while the game is in <code>setup</code>.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/admin/games/{code}/users/{userId}/impersonate</code>
        </header>
        <p>Return the target player's token so an admin can sign in as them.</p>
        <pre class="api-code">{ "token": "...", "userId": "uuid", "code": "abcd" }</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--delete">DELETE</span>
          <code class="api-path">/api/admin/games/{code}/questions/{questionId}</code>
        </header>
        <p>Remove a question. Only allowed in <code>setup</code>.</p>
      </article>

      <h3>Authoring a Company Consensus set</h3>
      <p>
        The next four endpoints are <strong>poll-only</strong> and
        <strong>setup-only</strong>. In a classic game the questions belong to
        the players who wrote them, and once teams are playing, changing the
        set would orphan their answers — both cases return <code>400</code>.
      </p>
      <p>
        They all take the same question shape: a text plus exactly five
        answers, each with the number of survey respondents who gave it.
        Options are stored shuffled, so the highest-scoring answer never sits
        in a predictable slot. The admin views always redisplay them ranked by
        points, which is how survey results arrive; the stored order is never
        surfaced.
      </p>
      <pre class="api-code">{
  "text": "What keeps this office running?",
  "answers": [
    { "text": "Coffee",        "points": 41 },
    { "text": "Tea",           "points": 27 },
    { "text": "Energy drinks", "points": 12 },
    { "text": "Tap water",     "points": 9  },
    { "text": "Sheer spite",   "points": 5  }
  ]
}</pre>
      <p>
        Answer text must be non-empty and unique within the question, and
        points must not be negative.
      </p>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/questions</code>
        </header>
        <p>Append one question to the set. Returns the created <code>Question</code>.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--put">PUT</span>
          <code class="api-path">/api/admin/games/{code}/questions/{questionId}</code>
        </header>
        <p>
          Replace one question, keeping its slot in the running order. Returns
          the updated <code>Question</code>, or <code>404</code> if the id does
          not belong to this game.
        </p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/questions/{questionId}/move</code>
        </header>
        <p>
          Shift a question one slot in the running order. Poll sets are played
          in the order the host arranged them, so this is how the warm-up gets
          to the front. Responds <code>204</code> with no body.
        </p>
        <pre class="api-code">{ "direction": "up" | "down" }</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/admin/games/{code}/questions/import</code>
        </header>
        <p>
          Replace the entire set from one payload — the bulk path behind the
          console's "Bulk import from JSON". Maximum 50 questions; the whole
          payload is validated before anything is written, so a bad entry
          leaves the existing set untouched.
        </p>
        <pre class="api-code">{ "questions": [ { "text": "...", "answers": [ ... ] } ] }</pre>
        <p><strong>200 response:</strong></p>
        <pre class="api-code">{ "imported": 12, "questions": [ ... ] }</pre>
      </article>
    </section>

    <!-- ---------- Player ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>Player</h2>
      <p>
        Routes marked <em>auth</em> require
        <code>X-Player-Token: &lt;token&gt;</code>. The rest are public.
      </p>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/games/{code}</code>
        </header>
        <p>Lightweight lookup used by the landing page. Returns <code>404</code> if the code is unknown.</p>
        <pre class="api-code">{
  "code": "abcd",
  "name": "Game night",
  "state": "setup",
  "scheduledAt": null
}</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/games/{code}/join</code>
        </header>
        <p>Register as a player. The game must be in <code>setup</code>. If <code>email</code> is provided, a magic-link rejoin email is sent asynchronously.</p>
        <pre class="api-code">{
  "name": "Ada",
  "photoImageId": "uuid",       // optional — see /api/images
  "email": "ada@example.com"    // optional
}</pre>
        <p><strong>200 response:</strong></p>
        <pre class="api-code">{
  "token":  "deadbeef...",
  "userId": "uuid",
  "gameId": "uuid",
  "code":   "abcd"
}</pre>
        <p>Returns <code>409</code> if the display name is already taken.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/me</code>
          <span class="api-pill">auth</span>
        </header>
        <p>Resolve the player token to its user and the game they belong to.</p>
        <pre class="api-code">{ "user": User, "game": Game }</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--put">PUT</span>
          <code class="api-path">/api/me</code>
          <span class="api-pill">auth</span>
        </header>
        <p>Update the current player's name, photo, or email. Body shape matches the join request. A freshly set or changed email triggers a magic-link send.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/games/{code}/users</code>
        </header>
        <p>Public roster (no tokens or emails).</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/games/{code}/questions</code>
        </header>
        <p>
          List the game's questions. The <code>correct</code> field is
          included only when the game state is <code>finished</code>. For
          <code>poll</code> questions the per-option <code>points</code> are
          stripped under the same rule — the survey counts <em>are</em> the
          answer, so shipping them early would hand every team a perfect
          score.
        </p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--put">PUT</span>
          <code class="api-path">/api/games/{code}/questions</code>
          <span class="api-pill">auth</span>
        </header>
        <p>
          Create or update the calling player's question. Only allowed in
          <code>setup</code>, and only in <code>classic</code> games — in a
          poll game the host owns the whole set.
        </p>
        <pre class="api-code">{
  "text": "Which year was the Eiffel Tower completed?",
  "photoImageId": "uuid",         // required
  "answerType": "yesno" | "choice" | "number",
  "options": ["...","..."],       // 2-4 strings for "choice", omitted/[] otherwise
  "correct": "yes" | 0 | 1889     // see "Answer types" below
}</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/games/{code}/leaderboard</code>
        </header>
        <p>Sorted array of <code>Score</code> rows.</p>
        <pre class="api-code">[
  { "userId": "uuid", "userName": "Ada", "photoImageId": "uuid",
    "points": 540, "correct": 4 }
]</pre>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/ai/suggest</code>
        </header>
        <p>Ask Anthropic Claude to draft a trivia question. The optional photo is supplied as the <code>medium</code> variant of an already-uploaded image.</p>
        <pre class="api-code">{
  "hint": "this is a picture of my dog",
  "answerType": "yesno" | "choice" | "number",
  "photoImageId": "uuid"          // optional
}</pre>
        <p><strong>200 response:</strong></p>
        <pre class="api-code">{
  "text": "...",
  "options": ["Yes", "No"],
  "correct": "yes"
}</pre>
        <p>Returns <code>502</code> if the upstream call fails.</p>
      </article>
    </section>

    <!-- ---------- Images ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>Images</h2>
      <p>
        Images are content-addressed: uploading the same bytes twice returns
        the same UUID. The upload path is unauthenticated; possession of the
        UUID is the capability for reads. Variants are pre-rendered at write
        time. Originals and variants both ship with
        <code>Cache-Control: public, max-age=31536000, immutable</code> and
        respond to <code>If-None-Match</code>.
      </p>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--post">POST</span>
          <code class="api-path">/api/images</code>
        </header>
        <p>
          <code>multipart/form-data</code> with a single <code>file</code>
          field. Max 8 MiB. EXIF is stripped; the image is re-encoded as JPEG.
        </p>
        <pre class="api-code">{ "id": "uuid" }</pre>
        <p>Returns <code>413</code> if the upload is too large.</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/images/{id}</code>
        </header>
        <p>Original image bytes (<code>image/jpeg</code>).</p>
      </article>

      <article class="api-ep">
        <header class="api-ep__hdr">
          <span class="api-method api-method--get">GET</span>
          <code class="api-path">/api/images/{id}/{variant}</code>
        </header>
        <p>
          Variant is one of <code>thumb</code> (≤ 128 px) or
          <code>medium</code> (≤ 640 px). Anything else returns
          <code>404</code>.
        </p>
      </article>
    </section>

    <!-- ---------- WebSocket ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>WebSocket</h2>
      <p>
        Live game state is pushed over a single endpoint. The role is selected
        by query parameters at handshake time:
      </p>
      <pre class="api-code">// Player:
GET /ws?token=&lt;playerToken&gt;

// Admin:
GET /ws?role=admin&amp;token=&lt;adminJWT&gt;&amp;code=&lt;gameCode&gt;

// Board (the projector view at /g/{code}/board):
GET /ws?role=board&amp;code=&lt;gameCode&gt;</pre>
      <p>
        A board connection carries <strong>no token</strong>: a TV in the room
        is not a participant, so it gets no player identity and can only
        listen. It receives the player-facing view, which means poll option
        points stay hidden until the reveal — nobody is holding the screen, but
        everybody can see it.
      </p>

      <h3>Inbound (client → server)</h3>
      <p>JSON envelopes shaped as <code>{ "type": ..., "data": ... }</code>.</p>
      <ul>
        <li>
          <code>answer</code> — submit an answer. Player role only.
          <pre class="api-code">{
  "type": "answer",
  "data": { "questionId": "uuid", "value": &lt;yesno | choice/poll index | number&gt; }
}</pre>
          Answers are silently dropped if the question is no longer active,
          if the player already answered, or if the response exceeded the
          per-question timeout.
        </li>
        <li>
          <code>ping</code> — server replies with <code>{ "type": "pong" }</code>.
        </li>
      </ul>

      <h3>Outbound (server → client)</h3>
      <ul>
        <li>
          <code>gameState</code> — the full game view. Pushed on join, on any
          admin action, and on auto-close. Includes <code>serverNow</code> so
          clients can correct for local clock skew when computing the
          countdown. The <code>correct</code> field on the active question is
          present only for admins, or for everyone once
          <code>questionState === "revealed"</code>.
        </li>
        <li>
          <code>users</code> — array of <code>User</code> rows. Broadcast on
          join/update/delete.
        </li>
        <li>
          <code>questionsAdmin</code> — full <code>Question</code> list,
          including <code>correct</code>. Admin clients only.
        </li>
        <li>
          <code>presence</code> — <code>{ online: string[] }</code>. Pushed to
          admins when players connect or disconnect.
        </li>
        <li>
          <code>answerAck</code> — confirmation that the calling player's
          answer was recorded. Replayed on reconnect so a refresh mid-question
          lands on the "locked in" view.
          <pre class="api-code">{ "type": "answerAck",
  "data": { "questionId": "uuid", "responseMs": 1234 } }</pre>
        </li>
        <li>
          <code>playerAnswered</code> — notification that a player submitted
          (no value, just metadata). Sent to admins and to boards, which use it
          to light up each team's name as it locks in.
        </li>
        <li>
          <code>answeredSnapshot</code> — board-only, sent once on connect.
          Lists who has already answered the active question, so a TV that
          refreshes mid-question comes back with the right names lit instead of
          a blank row.
          <pre class="api-code">{ "type": "answeredSnapshot",
  "data": { "questionId": "uuid", "userIds": ["uuid", "..."] } }</pre>
        </li>
        <li>
          <code>gameDeleted</code> — admin deleted the game; the client
          should disconnect.
        </li>
      </ul>
    </section>

    <!-- ---------- Types ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>Data types</h2>

      <h3>Game</h3>
      <pre class="api-code">{
  "id": "uuid",
  "code": "abcd",
  "name": "Game night",
  "mode": "classic" | "poll",
  "state": "setup" | "game" | "finished",
  "hideLeaderboardTail": true,
  "currentQuestionId": "uuid" | null,
  "questionState": "idle" | "active" | "revealed",
  "questionStartedAt": "RFC3339" | null,
  "questionClosedAt":  "RFC3339" | null,
  "questionTimeoutSeconds": 30,
  "scheduledAt": "RFC3339" | null,
  "createdAt":   "RFC3339"
}</pre>

      <h3>User</h3>
      <pre class="api-code">{
  "id": "uuid",
  "gameId": "uuid",
  "name": "Ada",
  "photoImageId": "uuid" | null,
  "email": "ada@example.com" | "",
  "token": "deadbeef...",   // omitted on public endpoints
  "createdAt": "RFC3339",
  "lastSeen":  "RFC3339"
}</pre>

      <h3>Question</h3>
      <pre class="api-code">{
  "id":     "uuid",
  "gameId": "uuid",
  "userId": "uuid" | null,  // null for host-authored "poll" questions
  "text":   "...",
  "photoImageId": "uuid" | null,
  "answerType": "yesno" | "choice" | "number" | "poll",
  "options":    [...],      // see below; empty for "yesno" and "number"
  "correct":    &lt;any&gt;,  // only present when revealed/finished/admin
  "sortOrder":  0,
  "createdAt":  "RFC3339"
}</pre>
      <p>
        <code>options</code> is an array of strings for <code>choice</code>.
        For <code>poll</code> it is an array of <code>PollOption</code>, whose
        <code>points</code> field is withheld until the same moment
        <code>correct</code> is:
      </p>
      <pre class="api-code">{
  "text":   "Coffee",
  "points": 41        // omitted before reveal
}</pre>

      <h3>Answer</h3>
      <pre class="api-code">{
  "id": "uuid",
  "questionId": "uuid",
  "userId":     "uuid",
  "answer":     &lt;any&gt;,
  "responseMs": 1234,
  "isCorrect":  true,
  "points":     142,
  "createdAt":  "RFC3339"
}</pre>
    </section>

    <!-- ---------- Answer types & scoring ---------- -->
    <section class="card stack legal-prose api-prose">
      <h2>Answer types &amp; scoring</h2>

      <h3>Answer encoding</h3>
      <ul>
        <li><code>yesno</code> — <code>options</code> is ignored; <code>correct</code> is the string <code>"yes"</code> or <code>"no"</code>.</li>
        <li><code>choice</code> — <code>options</code> is 2–4 strings; <code>correct</code> is the 0-based index of the right option.</li>
        <li><code>number</code> — <code>options</code> is <code>[]</code>; <code>correct</code> is a JSON number. Player submissions are also JSON numbers.</li>
        <li>
          <code>poll</code> — <code>options</code> is exactly 5
          <code>PollOption</code> entries; <code>correct</code> is
          <code>null</code>, because no single answer is right. Submissions are
          the 0-based index of the chosen option, same as <code>choice</code>.
        </li>
      </ul>

      <h3>Base points</h3>
      <ul>
        <li>yes/no — <code>100</code></li>
        <li>choice, 2 options — <code>100</code>; 3 options — <code>200</code>; 4 options — <code>300</code>, plus <code>100</code> per option above 4</li>
        <li>number — <code>300</code></li>
        <li>
          poll — no fixed base. The chosen option's own <code>points</code>
          value is the base, so every option scores and the crowd's favourite
          pays most. An option worth <code>0</code> scores nothing.
        </li>
      </ul>

      <h3>Time bonus</h3>
      <p>
        Correct yes/no, choice and poll answers earn an additional time bonus
        that decays linearly from <code>base / 2</code> at <code>0 ms</code> to
        <code>0</code> at the game's own
        <code>questionTimeoutSeconds</code> — so a 90 s question rewards speed
        across the whole 90 s, not just the first 30. Answers that arrive after
        that window are rejected at the WebSocket layer.
      </p>

      <h3>Number scoring</h3>
      <p>
        Number answers are scored at reveal time across the whole field. A
        guess within an absolute tolerance of <code>max(|c| × 0.005, 1)</code>
        is treated as exact and earns <code>base + timeBonus</code>. The top
        three non-exact guesses receive partial credit weighted by rank
        (<code>1.0</code>, <code>0.66</code>, <code>0.33</code>) and closeness
        <code>max(0, 1 − diff / scale)</code>, where
        <code>scale = max(|c| × 0.5, 10)</code>. Ties on distance are broken by
        faster response time.
      </p>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/" class="btn-link">← Back to home</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase" class="btn-link">Implementation showcase</RouterLink>
      <span aria-hidden="true">·</span>
      <a class="btn-link" href="https://github.com/oglimmer/trivia" target="_blank" rel="noopener noreferrer">Source on GitHub</a>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
