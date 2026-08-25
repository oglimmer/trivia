const messages: Record<string, Record<string, string>> = {
  en: {
    heroEyebrow: "Two ways to play",
    heroMottoPrefix: "Game ",
    heroMottoEm: "night",
    heroMottoSuffix: "written by your people.",
    heroSubtitle: "Type the code your host shared.",
    gameCodeLabel: "Game code",
    gameCodePlaceholder: "abcd",
    continueButton: "Continue →",
    loadingButton: "Looking up…",
    hosting: "Hosting?",
    openAdmin: "Open admin →",
    errorGameNotFound: "No game with that code",

    formatsHeading: "Two formats",
    formatsLede: "Your host picks one when they create the game.",

    classicName: "Classic trivia",
    classicWho: "Everyone brings a photo and a question. The host runs the round.",
    classicCaption: "On your phone",
    classicQuestion: "Where was this photo taken?",
    classicOpt1: "Porto",
    classicOpt2: "Lisbon",
    classicOpt3: "Seville",
    classicOpt4: "Naples",
    classicRule: "One right answer. Answer early and the clock pays a bonus.",

    pollName: "Company Consensus",
    pollWho: "The host loads questions from a survey. Teams guess what most people said.",
    pollCaption: "On the projector",
    pollQuestion: "What keeps this office running?",
    pollOpt1: "Coffee",
    pollOpt2: "Tea",
    pollOpt3: "Energy drinks",
    pollOpt4: "Tap water",
    pollOpt5: "Sheer spite",
    pollRule: "Every answer scores. The more people who said it, the more it pays.",

    hostBoardNote: "Company Consensus games come with a board view for the TV.",
  },
  de: {
    heroEyebrow: "Zwei Arten zu spielen",
    heroMottoPrefix: "Spiel ",
    heroMottoEm: "Nacht",
    heroMottoSuffix: "geschrieben von euch.",
    heroSubtitle: "Gib den Code ein, den dein Gastgeber geteilt hat.",
    gameCodeLabel: "Spielcode",
    gameCodePlaceholder: "abcd",
    continueButton: "Weiter →",
    loadingButton: "Suche…",
    hosting: "Host?",
    openAdmin: "Admin öffnen →",
    errorGameNotFound: "Kein Spiel mit diesem Code",

    formatsHeading: "Zwei Formate",
    formatsLede: "Dein Gastgeber wählt eines beim Anlegen des Spiels.",

    classicName: "Klassisches Quiz",
    classicWho: "Jeder bringt ein Foto und eine Frage mit. Der Gastgeber leitet die Runde.",
    classicCaption: "Auf deinem Handy",
    classicQuestion: "Wo wurde dieses Foto gemacht?",
    classicOpt1: "Porto",
    classicOpt2: "Lissabon",
    classicOpt3: "Sevilla",
    classicOpt4: "Neapel",
    classicRule: "Eine richtige Antwort. Wer früh antwortet, bekommt einen Zeitbonus.",

    pollName: "Company Consensus",
    pollWho: "Der Gastgeber lädt Fragen aus einer Umfrage. Teams raten, was die meisten gesagt haben.",
    pollCaption: "Auf dem Beamer",
    pollQuestion: "Was hält dieses Büro am Laufen?",
    pollOpt1: "Kaffee",
    pollOpt2: "Tee",
    pollOpt3: "Energydrinks",
    pollOpt4: "Leitungswasser",
    pollOpt5: "Reiner Trotz",
    pollRule: "Jede Antwort gibt Punkte. Je mehr Leute sie nannten, desto mehr zählt sie.",

    hostBoardNote: "Company-Consensus-Spiele haben eine Board-Ansicht für den Fernseher.",
  },
  fr: {
    heroEyebrow: "Deux façons de jouer",
    heroMottoPrefix: "Soirée ",
    heroMottoEm: "jeux",
    heroMottoSuffix: "écrite par vos équipes.",
    heroSubtitle: "Tapez le code partagé par votre hôte.",
    gameCodeLabel: "Code du jeu",
    gameCodePlaceholder: "abcd",
    continueButton: "Continuer →",
    loadingButton: "Recherche…",
    hosting: "Héberger ?",
    openAdmin: "Ouvrir admin →",
    errorGameNotFound: "Aucun jeu avec ce code",

    formatsHeading: "Deux formats",
    formatsLede: "Votre hôte en choisit un à la création du jeu.",

    classicName: "Quiz classique",
    classicWho: "Chacun apporte une photo et une question. L'hôte mène la manche.",
    classicCaption: "Sur votre téléphone",
    classicQuestion: "Où cette photo a-t-elle été prise ?",
    classicOpt1: "Porto",
    classicOpt2: "Lisbonne",
    classicOpt3: "Séville",
    classicOpt4: "Naples",
    classicRule: "Une seule bonne réponse. Répondez vite, le chrono donne un bonus.",

    pollName: "Company Consensus",
    pollWho: "L'hôte importe les questions d'un sondage. Les équipes devinent la réponse la plus donnée.",
    pollCaption: "Sur le projecteur",
    pollQuestion: "Qu'est-ce qui fait tourner ce bureau ?",
    pollOpt1: "Le café",
    pollOpt2: "Le thé",
    pollOpt3: "Les boissons énergisantes",
    pollOpt4: "L'eau du robinet",
    pollOpt5: "La pure rancune",
    pollRule: "Chaque réponse rapporte. Plus elle a été donnée, plus elle vaut.",

    hostBoardNote: "Les jeux Company Consensus ont une vue tableau pour la télé.",
  },
}

function getLocale(): string {
  const lang = (navigator.language || 'en').split('-')[0].toLowerCase()
  return ['en', 'de', 'fr'].includes(lang) ? lang : 'en'
}

export function useI18n() {
  const locale = getLocale()
  const t = (key: string): string => messages[locale][key] ?? messages.en[key] ?? key
  return { t, locale }
}
