<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">★</span>
      <span class="hero__eyebrow">Showcase · 02</span>
      <h1 class="hero__title">Images /<br /><em>content-addressed, dedup-friendly</em></h1>
      <p class="hero__subtitle">
        Uploads are re-encoded to JPEG, hashed, deduped, and stored alongside
        two pre-rendered variants — all in Postgres, no object store.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>At a glance</h2>
      <ul>
        <li>Uploads decoded, re-encoded as JPEG (this drops EXIF), and hashed with SHA-256.</li>
        <li>SHA-256 is the dedup key — two players uploading the same photo share one row.</li>
        <li>Two variants (<code>thumb</code> ≤ 128 px, <code>medium</code> ≤ 640 px) are pre-rendered at write time, in the same transaction as the original.</li>
        <li>Reads are unauthenticated — possession of the UUID <em>is</em> the capability.</li>
        <li>A background sweep deletes orphans (images no user or question references) after a 1-hour grace period.</li>
        <li>Only classic games use question photos. A Company Consensus set is text-only, so a poll game touches this subsystem exactly once — for player avatars.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Key files</h2>
      <ul>
        <li><code>backend/internal/images/images.go</code> — encode/hash pipeline + DB I/O.</li>
        <li><code>backend/internal/api/images.go</code> — HTTP upload/serve + ETag + cache headers.</li>
        <li><code>backend/internal/api/server.go</code> — <code>RunOrphanImageGC</code> (the sweep loop).</li>
        <li><code>backend/migrations/0007_images.sql</code> — <code>images</code> + <code>image_variants</code> tables.</li>
        <li><code>backend/migrations/0008_drop_photo_b64.sql</code> — drops the old inline-base64 column.</li>
      </ul>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Why store bytes in Postgres?</h2>
      <p>
        The deployment target is a single small node with a persistent volume.
        Adding an S3-compatible blob store would mean another service to run,
        another credential to rotate, another backup story. Postgres is already
        present and already has durable storage; for the sizes involved (8 MiB
        per upload, dozens per game), <code>BYTEA</code> is plenty fast.
      </p>
      <p>
        The trade-off shows up only at backup time: <code>pg_dump</code> grows.
        If the deployment ever needs to scale to thousands of concurrent games,
        moving bytes out is a single-table change — the API contract above
        (UUID URL, content-addressed) stays identical.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>The encode pipeline</h2>
      <p>
        Pure function, no database. Easy to unit-test (see
        <code>images_test.go</code>).
      </p>
      <pre class="api-code">// backend/internal/images/images.go
func process(raw []byte) (*processed, error) {
    img, _, err := image.Decode(bytes.NewReader(raw))
    if err != nil { return nil, fmt.Errorf("decode: %w", err) }

    canonical, err := encodeJPEG(img, OriginalQuality) // q=85
    if err != nil { return nil, fmt.Errorf("encode original: %w", err) }
    sum := sha256.Sum256(canonical)

    thumb,  _ := makeVariant(img, ThumbMaxEdge)   // 128 px max edge
    medium, _ := makeVariant(img, MediumMaxEdge)  // 640 px max edge
    // ...
}</pre>
      <p>
        Decoding accepts JPEG/PNG/GIF; the canonical form is always JPEG. EXIF
        orientation, GPS, and camera metadata don't survive the round-trip,
        which is the point. Variant scaling uses Lanczos resampling via the
        <code>imaging</code> package.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Dedup &amp; the upload race</h2>
      <p>
        After hashing, the writer takes the fast path: if a row with this
        <code>sha256</code> already exists, return its id without writing
        anything. This is what makes the "same photo, twice" case cheap.
      </p>
      <pre class="api-code">var existingID string
err = s.pool.QueryRow(ctx, `SELECT id FROM images WHERE sha256=$1`, p.sha).Scan(&amp;existingID)
if err == nil {
    return existingID, nil
}</pre>
      <p>
        On a miss, the writer opens a transaction and inserts the row + both
        variants atomically. The interesting part is the <code>ON CONFLICT</code>:
      </p>
      <pre class="api-code">var id string
err = tx.QueryRow(ctx, `
    INSERT INTO images (sha256, mime, width, height, bytes, byte_size)
    VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (sha256) DO UPDATE SET sha256 = images.sha256
    RETURNING id
`, p.sha, jpegMime, p.width, p.height, p.canonical, len(p.canonical)).Scan(&amp;id)</pre>
      <p>
        Two clients uploading the same bytes concurrently both miss the fast
        path. Without the conflict clause, the loser would get
        <code>SQLSTATE 23505</code> and have to retry. The
        no-op-update-RETURNING trick lets the loser observe the winner's id in
        one query — race resolved, no retry loop.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Serving + caching</h2>
      <p>
        Because bytes for a given id never change, the response is
        aggressively cacheable. The handler sets a 1-year
        <code>immutable</code> directive and an ETag derived from the SHA:
      </p>
      <pre class="api-code">// backend/internal/api/images.go
const imageCacheControl = "public, max-age=31536000, immutable"

func serveBlob(w http.ResponseWriter, r *http.Request, b *images.Blob) {
    w.Header().Set("ETag", b.ETag)
    w.Header().Set("Cache-Control", imageCacheControl)
    if match := r.Header.Get("If-None-Match"); match != "" &amp;&amp; match == b.ETag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    w.Header().Set("Content-Type", b.Mime)
    _, _ = w.Write(b.Bytes)
}</pre>
      <p>
        Variant ETags are <code>"&lt;sha&gt;-thumb"</code> /
        <code>"&lt;sha&gt;-medium"</code> so caches can keep all three responses
        without collision.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Orphan sweep</h2>
      <p>
        Uploads happen <em>before</em> a join — the client gets a UUID and
        only then submits the form that attaches it to a user or question. If
        the user abandons the flow, the image is orphaned. A background loop
        cleans them up:
      </p>
      <pre class="api-code">// backend/internal/api/server.go
const orphanImageGCInterval = 15 * time.Minute
const orphanImageGrace      = 1 * time.Hour

func (s *Server) RunOrphanImageGC(ctx context.Context) {
    t := time.NewTicker(orphanImageGCInterval)
    defer t.Stop()
    for {
        select {
        case &lt;-ctx.Done():        return
        case now := &lt;-t.C:
            s.deleteOrphanImages(ctx, now.Add(-orphanImageGrace))
        }
    }
}</pre>
      <p>
        The DELETE itself is a single SQL statement that filters by
        <em>not-referenced</em> and <em>old enough</em>:
      </p>
      <pre class="api-code">DELETE FROM images i
WHERE i.created_at &lt; $1
  AND NOT EXISTS (SELECT 1 FROM users     u WHERE u.photo_image_id = i.id)
  AND NOT EXISTS (SELECT 1 FROM questions q WHERE q.photo_image_id = i.id);</pre>
      <p>
        The 1-hour grace is what protects against racing the
        upload-then-attach flow on a slow client. <code>image_variants</code>
        rows go away via <code>ON DELETE CASCADE</code>.
      </p>
    </section>

    <section class="card stack legal-prose api-prose">
      <h2>Design choices &amp; gotchas</h2>
      <ul>
        <li>
          <strong>EXIF is stripped, not redacted.</strong> The re-encode pass
          means we cannot accidentally leak a GPS tag — there's no code path
          that copies the original blob through.
        </li>
        <li>
          <strong>Two caps on upload size.</strong>
          <code>http.MaxBytesReader</code> at the transport layer (fails fast,
          returns 413) plus <code>io.LimitReader(...,&nbsp;MaxUploadBytes+1)</code>
          in the package — defense in depth.
        </li>
        <li>
          <strong>FK is <code>ON DELETE SET NULL</code>, not CASCADE.</strong>
          Deleting an image must not cascade into deleting the user or
          question that referenced it; the GC and the user-delete flow handle
          orphans separately.
        </li>
        <li>
          <strong>No background re-encoding.</strong> Variants must exist before
          the upload returns. This trades a slower POST for a faster page paint
          — the client never has to ask "is the thumb ready yet?".
        </li>
      </ul>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/developers-showcase" class="btn-link">← All showcases</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/developers-showcase/database" class="btn-link">Next: Database →</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
