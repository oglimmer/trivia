<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__eyebrow">Legal</span>
      <h1 class="hero__title"><em>Privacy</em><br />Policy</h1>
      <p class="hero__subtitle">Last updated: {{ lastUpdated }}</p>
    </section>

    <section class="card stack legal-prose">
      <p>
        This Privacy Policy explains how oglimmer.com / Oliver Zimpasser ("we",
        "our", "us") processes personal data when you use Trivia (the "Platform").
        It applies to all visitors, players and hosts.
      </p>

      <h2>1. Controller</h2>
      <p>
        Controller within the meaning of Art. 4(7) GDPR:<br />
        Oliver Zimpasser, Berliner Str. 39a, 63110 Rodgau, Germany<br />
        Email: <a href="mailto:oglimmer@gmail.com">oglimmer@gmail.com</a>
      </p>
      <p>
        We have not appointed a Data Protection Officer because we are not required to do so under
        Art. 37 GDPR.
      </p>

      <h2>2. Data We Process</h2>
      <p>We process the following categories of personal data, depending on how you use the Platform:</p>
      <ul>
        <li>
          <strong>Player profile data</strong> you enter: display name (required to join a game),
          profile photo (optional) and email address (optional).
        </li>
        <li>
          <strong>Game content</strong> you submit: trivia questions, answer options, the correct
          answer, and any photo you upload to accompany a question.
        </li>
        <li>
          <strong>Game state and play history</strong>: the answers you give, your response times and
          your scores for the games you join.
        </li>
        <li>
          <strong>Host session data</strong>: a stateless session token (JSON Web Token) issued to a
          host after they authenticate with the admin password. The admin password itself is configured
          on the server as an environment variable and is never written to our database.
        </li>
        <li>
          <strong>Technical data</strong>: each HTTP request to our backend is logged with the request
          method, path, HTTP status, response time and the remote IP address. These access logs are
          written to the application's standard output. We do not log the User-Agent header or any
          other request headers, and we do not use device fingerprinting.
        </li>
      </ul>

      <h2>3. Whether You Have to Provide Data</h2>
      <p>
        You are not under a statutory or contractual obligation to provide personal data to us. The
        display name is technically necessary to join a game. The profile photo and email address are
        optional; if you do not provide an email address, we will not be able to send you a magic-link
        login (see Section 5) and you will need to stay on the device on which you originally joined
        the game.
      </p>

      <h2>4. Purposes and Legal Bases</h2>
      <ul>
        <li>
          <strong>Operating the game</strong> (Art. 6(1)(b) GDPR): running games, real-time updates
          over WebSocket, scoring, and showing your card (display name and, if you provided one,
          profile photo) to the other players in the same game.
        </li>
        <li>
          <strong>Magic-link rejoin</strong> (Art. 6(1)(b) GDPR): if you provide an email address, we
          send a one-click login link so you can rejoin the same game from another device. The email
          address is used solely for this purpose and is not used for marketing, profiling or any
          other communication.
        </li>
        <li>
          <strong>Host authentication</strong> (Art. 6(1)(b) and (f) GDPR): verifying the admin
          password and issuing a session token so that hosts can manage their games.
        </li>
        <li>
          <strong>Security, abuse prevention and troubleshooting</strong> (Art. 6(1)(f) GDPR): our
          legitimate interest is to keep the service running, secure and free of misuse. To that end
          we write basic HTTP access logs (see Section 2) and retain them for a short period.
        </li>
        <li>
          <strong>Compliance with legal obligations</strong> (Art. 6(1)(c) GDPR): where applicable law
          requires us to retain or disclose data.
        </li>
      </ul>
      <p>
        We do not carry out any automated decision-making, including profiling, that produces legal
        effects or similarly significantly affects you within the meaning of Art. 22 GDPR. Scoring
        within a game is the deterministic application of the host-defined correct answer and is not
        used for any decision outside that game.
      </p>

      <h2>5. AI-Assisted Question Suggestions</h2>
      <p>
        The host interface includes an optional "Help me with AI" button which calls the Anthropic
        API (operated by Anthropic, PBC, San Francisco, USA) to suggest trivia questions. Only the
        prompt text you type and, where you have chosen one, the question photo are sent to Anthropic
        for that single request. No display names, email addresses, game history, scores or other
        player data are transmitted. This feature is only active when the operator of the instance has
        configured an Anthropic API key.
      </p>

      <h2>6. Recipients of Your Data</h2>
      <p>We disclose personal data only to the following categories of recipients:</p>
      <ul>
        <li>
          <strong>Other players in the same game</strong>: your display name and, if you uploaded one,
          your profile photo are shown to every other person who has joined that game. Other players
          are independent data subjects, not our processors; once they see your card we cannot
          technically prevent them from remembering it.
        </li>
        <li>
          <strong>Hosting and infrastructure providers</strong> on whose servers this instance of the
          Platform is operated; they act as processors under Art. 28 GDPR.
        </li>
        <li>
          <strong>Our SMTP/email relay</strong>, configured by the operator of the instance, used
          solely to deliver magic-link emails. Acts as a processor under Art. 28 GDPR.
        </li>
        <li>
          <strong>Anthropic, PBC</strong>, where the operator has enabled the optional AI feature and
          a host triggers it; see Section 5. Anthropic acts as a processor under Art. 28 GDPR for
          this request.
        </li>
      </ul>
      <p>
        Uploaded photos and game content are stored in our own PostgreSQL database on the same
        infrastructure as the rest of the Platform; they are <em>not</em> handed off to a third-party
        image host or object store.
      </p>

      <h2>7. International Transfers</h2>
      <p>
        Calls to the Anthropic API result in a transfer of the prompt text (and, where used, the
        question photo) to the United States. We rely on the EU‑US Data Privacy Framework adequacy
        decision of 10 July 2023 where the recipient is certified under it, and on the EU Standard
        Contractual Clauses (Art. 46(2)(c) GDPR) otherwise. Your SMTP/email relay may also be located
        outside the EU/EEA, depending on how the operator of this instance has configured it; the
        same safeguards apply. You can request a copy of the safeguards we rely on by contacting us
        (see Section 12).
      </p>

      <h2>8. Local Storage (No Cookies)</h2>
      <p>
        The Platform does not set any cookies. We only use the browser's <code>localStorage</code> to
        keep you signed in to the service you are actively using:
      </p>
      <ul>
        <li>
          <code>playerToken</code> — an opaque random token identifying you as a player in a specific
          game. Stored when you join a game.
        </li>
        <li>
          <code>adminToken</code> — a JSON Web Token identifying a host who has signed in with the
          admin password.
        </li>
      </ul>
      <p>
        Both items are strictly necessary to provide the service you have requested and therefore do
        not require consent under § 25 (2) Nr. 2 TTDSG. We do not use analytics, advertising or
        cross-site tracking technologies of any kind. You can delete these items at any time via
        your browser; doing so will sign you out.
      </p>

      <h2>9. Retention and Deletion</h2>
      <ul>
        <li>
          <strong>Player profile, photos, questions, answers and scores</strong>: retained for as
          long as the game exists. When the host deletes the game, all associated player profiles,
          uploaded photos, questions, answers and scores are deleted from our database. Photos that
          are no longer referenced by any question are removed by an orphan-cleanup routine.
        </li>
        <li>
          <strong>Server access logs</strong>: retained for a short period (typically a few days)
          unless we have a concrete reason to keep them longer to investigate an incident.
        </li>
        <li>
          <strong>Backups</strong>: where the operator of the instance maintains database backups,
          deleted data may persist in those backups until they are rotated in the ordinary course.
        </li>
      </ul>
      <p>
        You can ask us at any time to remove your profile, your uploaded photo or your email address
        from a game (Section 11). If you want all data associated with you removed, ask the host to
        delete the game, or contact us directly and we will arrange deletion.
      </p>

      <h2>10. Security</h2>
      <p>
        We implement appropriate technical and organisational measures within the meaning of
        Art. 32 GDPR to protect your data, including transport encryption (HTTPS), authenticated
        sessions and minimisation of the data we collect. No online service can guarantee absolute
        security.
      </p>

      <h2>11. Your Rights</h2>
      <p>Under the GDPR you have the following rights regarding your personal data:</p>
      <ul>
        <li>Right of access (Art. 15 GDPR).</li>
        <li>Right to rectification of inaccurate data (Art. 16 GDPR).</li>
        <li>Right to erasure ("right to be forgotten") (Art. 17 GDPR).</li>
        <li>Right to restriction of processing (Art. 18 GDPR).</li>
        <li>Right to data portability for data you have provided to us (Art. 20 GDPR).</li>
      </ul>
      <p>
        <strong>Right to object (Art. 21 GDPR).</strong> You have the right to object, on grounds
        relating to your particular situation, to processing of your personal data that is based on
        our legitimate interests (Art. 6(1)(f) GDPR), in particular our access-log processing
        described in Sections 2 and 4. If you object, we will stop processing the data in question
        unless we can demonstrate compelling legitimate grounds that override your interests or the
        processing is needed to establish, exercise or defend legal claims.
      </p>
      <p>
        To exercise any of these rights, contact us at
        <a href="mailto:oglimmer@gmail.com">oglimmer@gmail.com</a>. You also have the right to lodge
        a complaint with a supervisory authority. The competent authority for the controller is the
        Hessischer Beauftragter für Datenschutz und Informationsfreiheit, Wiesbaden, Germany; you may
        also lodge a complaint with the supervisory authority of your habitual residence or place of
        work.
      </p>

      <h2>12. Children</h2>
      <p>
        The Platform is not intended for children under 16. Do not create a player profile or upload
        content if you are under 16.
      </p>

      <h2>13. Changes to this Policy</h2>
      <p>
        We may update this Privacy Policy to reflect changes in our practices or legal requirements.
        The "Last updated" date at the top of this page reflects the effective date of the current
        version.
      </p>

      <h2>14. Contact</h2>
      <p>
        For privacy questions or requests, contact:
        <a href="mailto:oglimmer@gmail.com">oglimmer@gmail.com</a>.
      </p>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/" class="btn-link">← Back to home</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/terms" class="btn-link">Terms</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/imprint" class="btn-link">Imprint</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'

const lastUpdated = '2026-05-15'
</script>
