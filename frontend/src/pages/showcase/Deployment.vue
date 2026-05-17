<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 07</span>
      <h1 class="hero__title">Deployment /<br /><em>one chart, three workloads</em></h1>
      <p class="hero__subtitle">
        A single Helm chart ships the frontend, backend, optional bundled
        Postgres, and a Traefik 3 Ingress fronting all of it — secrets sealed
        into the repo with bitnami SealedSecrets.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>One chart in <code>helm/trivia</code> renders every Kubernetes object.</li>
        <li>Three Deployments: <code>api</code> (Go), <code>web</code> (nginx + SPA), and optionally <code>postgres</code>.</li>
        <li><code>Recreate</code> rollout strategy on the backend — the WebSocket hub is process-local, so two replicas would split clients.</li>
        <li>Secrets are SealedSecrets, committable to git; the controller unseals them in-cluster.</li>
        <li>Ingress is Traefik 3 + cert-manager (DNS-01); WebSocket upgrades work out of the box, no extra annotations needed.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>helm/trivia/Chart.yaml</code> — chart metadata, <code>appVersion</code>.</li>
        <li><code>helm/trivia/values.yaml</code> — every knob (images, SMTP, AI, ingress).</li>
        <li><code>helm/trivia/templates/deployment-backend.yaml</code> — env wiring + secret refs.</li>
        <li><code>helm/trivia/templates/ingress.yaml</code> — path-prefix routing.</li>
        <li><code>helm/trivia/templates/sealed-secret.yaml</code> — encrypted secret payload.</li>
        <li><code>helm/trivia/templates/postgres.yaml</code> — bundled DB with PVC.</li>
        <li><code>frontend/Dockerfile</code> / <code>backend/Dockerfile</code> — build outputs consumed by these manifests.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Shape of the deployment</h2>
      <pre class="api-code">                ┌──────────────────────────────────┐
                │   Traefik 3 (TLS terminator)     │
                └──────────────┬───────────────────┘
                               │
            ┌──────────────────┼──────────────────┐
            │ path = /api      │ path = /         │
            │ path = /ws       │   (catch-all)    │
            │ path = /health   │                  │
            │ path = /metrics  │                  │
            ▼                                      ▼
       ┌────────┐                            ┌────────┐
       │  api   │ ◄── env from SealedSecret  │  web   │   (nginx + Vue build)
       │ (Go)   │                            └────────┘
       └───┬────┘
           │ POSTGRES_*
           ▼
       ┌────────────┐
       │ postgres   │  (chart-bundled, PVC-backed)
       └────────────┘</pre>
      <p>
        Ingress path order matters: backend prefixes
        (<code>/api</code>, <code>/ws</code>, <code>/health</code>, <code>/metrics</code>) come
        <em>before</em> the frontend catch-all so they don't get swallowed by
        the SPA bundle.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Traefik 3 as the ingress controller</h2>
      <p>
        The cluster runs <strong>Traefik 3</strong> as its IngressClass — the
        chart emits a plain Kubernetes <code>Ingress</code> object and lets
        Traefik handle TLS termination, routing, and the WebSocket upgrade.
        TLS certificates are issued by cert-manager via the DNS-01 cluster
        issuer; the only annotation the Ingress strictly needs is the one
        pointing at it:
      </p>
      <pre class="api-code"># helm/trivia/values.yaml
ingress:
  enabled: true
  annotations:
    cert-manager.io/cluster-issuer: "oglimmer-com-dns"
  # className: "traefik"   # set if your cluster needs an explicit IngressClass</pre>
      <p>
        Three things about Traefik that the chart relies on without any extra
        config:
      </p>
      <ul>
        <li>
          <strong>WebSocket upgrades are automatic.</strong> Traefik forwards
          <code>Upgrade: websocket</code> and keeps the stream open for as
          long as the backend does — there is nothing equivalent to
          ingress-nginx's <code>proxy-read-timeout</code> annotation to set.
        </li>
        <li>
          <strong>No response buffering on streamed responses.</strong>
          Server-pushed frames reach the client live; the
          <code>proxy-buffering</code> dance from the nginx world doesn't
          apply.
        </li>
        <li>
          <strong>Body size limits live on the entrypoint.</strong> Per-Ingress
          body caps aren't an Ingress annotation in Traefik — if the
          entrypoint default isn't big enough for an AI-suggest payload
          (≤ 20 MiB in practice; image uploads are capped at 8 MiB in code),
          configure
          <code>entryPoints.websecure.transport.respondingTimeouts</code> and
          related fields on the Traefik install itself.
        </li>
      </ul>
      <p>
        Path order in <code>values.yaml</code> still matters for any
        Ingress controller: backend prefixes (<code>/api</code>,
        <code>/ws</code>, <code>/health</code>) come before the SPA
        catch-all <code>/</code>.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Secrets, sealed</h2>
      <p>
        Five secret keys are baked into <code>sealed-secret.yaml</code>:
      </p>
      <pre class="api-code">ADMIN_PASSWORD       # admin login (see Auth showcase)
JWT_SECRET           # HS256 signing key for admin JWTs
POSTGRES_PASSWORD    # the Postgres super-user password
ANTHROPIC_API_KEY    # AI suggestions; optional
SMTP_USER / SMTP_PASSWORD  # magic-link sender</pre>
      <p>
        The bitnami SealedSecrets controller decrypts them in-cluster into a
        plain <code>Secret</code> the Deployments mount as env vars. Because
        the encrypted blob is bound to the cluster's private key, the file is
        safe to commit — only this cluster can read it.
      </p>
      <p>
        The backend Deployment wires the unsealed secret into env vars:
      </p>
      <pre class="api-code">- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: trivia-secret        # rendered from the chart's helper
      key: ANTHROPIC_API_KEY
      optional: true             # AI is opt-in</pre>
      <p>
        <code>optional: true</code> on the Anthropic key means a deployment
        without it still boots — the AI endpoint just returns an error if
        called.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Recreate vs RollingUpdate</h2>
      <p>
        The backend rollout strategy is <code>type: Recreate</code>. With a
        process-local WebSocket hub (<RouterLink to="/developers-showcase/websocket">showcase</RouterLink>),
        a rolling update would have two backends briefly serving the same
        game, splitting players across hubs. The cost is a small downtime
        during deploys — measured in seconds for a Go binary — which the
        WebSocket reconnect loop on the client papers over.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Bundled vs external Postgres</h2>
      <p>
        The chart ships a single-replica Postgres on a 5 GiB PVC — fine for a
        hobby deployment. Set <code>postgres.enabled=false</code> and fill in
        <code>externalPostgres.host</code> to point at a managed DB instead;
        the password still comes from the sealed secret.
      </p>
      <pre class="api-code"># helm/trivia/values.yaml
postgres:
  enabled: true
  user: trivia
  database: trivia
  persistence:
    enabled: true
    size: 5Gi

externalPostgres:    # consulted only when postgres.enabled=false
  host: ""
  user: trivia
  database: trivia</pre>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Container security</h2>
      <pre class="api-code"># helm/trivia/values.yaml
securityContext:
  runAsNonRoot: true
  runAsUser:    10001
  runAsGroup:   10001
  capabilities:
    drop: [ALL]
  readOnlyRootFilesystem: false</pre>
      <p>
        The Go binary runs as UID 10001 with all Linux capabilities dropped.
        The root FS is intentionally writable because the binary writes nothing
        but logs to stdout — the flag is left as <code>false</code> so an
        operator can drop a debug file if they ever need to. Same posture on
        the nginx container (<code>frontendSecurityContext</code>).
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Observability</h2>
      <p>
        The backend exposes Prometheus metrics on a separate
        <code>/metrics</code> route mounted on the root mux — outside the chi
        router, the CORS middleware, and the request log — so scrapes don't
        appear in access logs and the endpoint isn't browser-CORS-exposed.
        It's guarded by a bearer token: requests without
        <code>Authorization: Bearer $METRICS_TOKEN</code> get
        <code>401</code>, and an unset token disables the endpoint entirely
        with a <code>404</code> (fail-closed beats accidentally shipping it
        open).
      </p>
      <pre class="api-code">curl -sH "Authorization: Bearer $METRICS_TOKEN" \
  https://trivia.example.com/metrics</pre>
      <p>
        Alongside the standard Go runtime + process collectors, the registry
        carries a set of app-specific series under the
        <code>trivia_</code> namespace:
      </p>
      <ul>
        <li>
          <strong>HTTP</strong> — <code>trivia_http_requests_total</code>,
          <code>trivia_http_request_duration_seconds</code>,
          <code>trivia_http_in_flight_requests</code>. Path is the chi route
          pattern (<code>/api/games/&#123;code&#125;</code>), not the raw URL,
          so game codes and image UUIDs don't blow up label cardinality.
          Instrumentation is scoped to <code>/api</code> only —
          <code>/ws</code> is excluded because the handler's lifetime is the
          WebSocket session, not request latency.
        </li>
        <li>
          <strong>WebSocket</strong> — <code>trivia_ws_connections</code>
          gauge labelled by role, incremented in the hub's
          <code>OnJoin</code>/<code>OnLeave</code> callbacks; plus
          <code>trivia_ws_session_duration_seconds</code> histogram (by role)
          observing each session's lifetime when the upgraded connection
          closes.
        </li>
        <li>
          <strong>Game lifecycle</strong> —
          <code>trivia_game_count</code> by state,
          <code>trivia_game_online_players</code>, plus counters for answers
          (by <code>result</code>), question activations, reveals, and
          auto-closes.
        </li>
        <li>
          <strong>AI &amp; images</strong> —
          <code>trivia_ai_suggest_requests_total</code> +
          <code>_duration_seconds</code>,
          <code>trivia_images_uploaded_total</code>,
          <code>trivia_images_orphans_deleted_total</code>.
        </li>
        <li>
          <code>trivia_build_info</code> — value always <code>1</code>;
          labels carry version, commit, build time, and Go version, so a
          single query identifies the deployed binary and a rise in
          <code>changes()</code> annotates redeploys.
        </li>
      </ul>
      <p>
        Live gauges that depend on state the metrics package can't see
        (online players, games by state) are pulled on every scrape via
        caller-supplied closures — so the <code>metrics</code> package
        stays free of <code>db</code> and <code>ws</code> dependencies.
        The Ingress exposes <code>/metrics</code> on the same host as
        <code>/api</code>; the Helm chart wires
        <code>METRICS_TOKEN</code> from the sealed secret as an optional
        key, so a deployment that never sets it simply leaves the endpoint
        disabled.
      </p>
      <p>
        One trap worth flagging: the HTTP middleware wraps the
        <code>ResponseWriter</code> to capture status codes for the request
        counter, so it has to forward <code>http.Hijacker</code> (and
        <code>http.Flusher</code>) to the underlying writer — otherwise the
        gorilla WebSocket upgrade fails with "response does not implement
        http.Hijacker" and every <code>/ws</code> call 500s.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>One chart, one namespace.</strong> No subcharts, no
          umbrella. The whole stack fits in one diff.
        </li>
        <li>
          <strong>Migrations run at backend boot.</strong> Not a
          <code>helm hook</code> — kept simple because migrations are
          idempotent (<RouterLink to="/developers-showcase/database">database showcase</RouterLink>).
          Add a pre-install Job if you ever scale past one replica.
        </li>
        <li>
          <strong>Image tags default to <code>latest</code>.</strong>
          <code>pullPolicy: Always</code> is set so re-deploys actually pull —
          but pin a digest in production.
        </li>
        <li>
          <strong>SMTP defaults are visible.</strong> Host/port/from live in
          plain values; only user/password are sealed. That keeps the rendered
          manifests readable in code review.
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers" class="btn-link">API reference</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
