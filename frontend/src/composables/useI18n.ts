const messages: Record<string, Record<string, string>> = {
  en: {
    heroEyebrow: "Bring-your-own-question",
    heroMotto: "Game <em>night</em>,<br />made by you.",
    heroSubtitle: "Type the code your host shared.",
    gameCodeLabel: "Game code",
    gameCodePlaceholder: "abcd",
    continueButton: "Continue →",
    loadingButton: "Looking up…",
    hosting: "Hosting?",
    openAdmin: "Open admin →",
  },
  de: {
    heroEyebrow: "Bring-deine-eigene-Frage",
    heroMotto: "Spiel <em>Nacht</em>,<br />gemacht von dir.",
    heroSubtitle: "Gib den Code ein, den dein Gastgeber geteilt hat.",
    gameCodeLabel: "Spielcode",
    gameCodePlaceholder: "abcd",
    continueButton: "Weiter →",
    loadingButton: "Suche…",
    hosting: "Host?",
    openAdmin: "Admin öffnen →",
  },
  fr: {
    heroEyebrow: "Apportez-votre-propre-question",
    heroMotto: "Soirée <em>jeux</em>,<br />faite par vous.",
    heroSubtitle: "Tapez le code partagé par votre hôte.",
    gameCodeLabel: "Code du jeu",
    gameCodePlaceholder: "abcd",
    continueButton: "Continuer →",
    loadingButton: "Recherche…",
    hosting: "Héberger ?",
    openAdmin: "Ouvrir admin →",
  },
}

function getLocale(): string {
  const lang = (navigator.language || 'en').split('-')[0]
  return ['en', 'de', 'fr'].includes(lang) ? lang : 'en'
}

export function useI18n() {
  const locale = getLocale()
  const t = (key: string): string => messages[locale][key] ?? messages.en[key] ?? key
  return { t, locale }
}
