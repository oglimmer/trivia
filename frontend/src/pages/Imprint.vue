<template>
  <main class="stack-lg legal-page" style="padding-top: 12px;">
    <section class="hero">
      <span class="hero__sparkle s1" aria-hidden="true">✦</span>
      <span class="hero__sparkle s2" aria-hidden="true">✦</span>
      <span class="hero__eyebrow">Legal</span>
      <h1 class="hero__title">Imprint /<br /><em>Impressum</em></h1>
      <p class="hero__subtitle">Legal information and contact details.</p>
    </section>

    <section class="card stack legal-prose">
      <p>For Germany: Angaben gemäß § 5 TMG</p>

      <h2>Diensteanbieter / Website Provider</h2>
      <p>Diese Webseite wird bereitgestellt von / This website is provided by:</p>

      <h3>Oliver Zimpasser</h3>
      <p>
        Berliner Str. 39a<br />
        63110 Rodgau<br />
        Germany
      </p>

      <h2>Kontakt / Contact</h2>
      <p>
        E‑Mail: <a href="mailto:oglimmer@gmail.com">oglimmer@gmail.com</a><br />
        (Angaben zur schnellen elektronischen Kontaktaufnahme und unmittelbaren Kommunikation gem. § 5 Abs. 1 Nr. 2 TMG)
      </p>

      <h2>Verantwortlich i.S.d. § 18 Abs. 2 MStV</h2>
      <p>
        Verantwortlich für den Inhalt / Responsible for content:<br />
        Oliver Zimpasser, Berliner Str. 39a, 63110 Rodgau, Germany
      </p>

      <h2>EU‑Streitschlichtung / Dispute Resolution</h2>
      <p>
        Die Europäische Kommission stellt eine Plattform zur Online‑Streitbeilegung (OS) bereit:
        <a href="https://ec.europa.eu/consumers/odr" target="_blank" rel="noopener">https://ec.europa.eu/consumers/odr</a>.
        Wir sind nicht verpflichtet und grundsätzlich nicht bereit, an Streitbeilegungsverfahren vor einer
        Verbraucherschlichtungsstelle teilzunehmen.
      </p>

      <h2>Service Information</h2>
      <p>All services are provided free of charge.</p>
    </section>

    <nav class="legal-nav">
      <RouterLink to="/" class="btn-link">← Back to home</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/privacy" class="btn-link">Privacy</RouterLink>
      <span aria-hidden="true">·</span>
      <RouterLink to="/terms" class="btn-link">Terms</RouterLink>
    </nav>
  </main>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
