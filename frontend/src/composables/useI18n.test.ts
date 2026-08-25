import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useI18n } from './useI18n'

// `navigator.language` is readonly, so it can only be swapped by redefining the
// property — the same trick the setup/teardown hooks use to put it back.
function setLanguage(lang: string) {
  Object.defineProperty(navigator, 'language', {
    value: lang,
    writable: true,
    configurable: true,
  })
}

describe('useI18n', () => {
  const originalLanguage = navigator.language

  beforeEach(() => setLanguage(originalLanguage))
  afterEach(() => setLanguage(originalLanguage))

  it('returns English translations when browser language is en', () => {
    setLanguage('en')
    const { t, locale } = useI18n()
    expect(locale).toBe('en')
    expect(t('heroEyebrow')).toBe('Bring-your-own-question')
  })

  it('composes the hero motto HTML correctly for English', () => {
    setLanguage('en')
    const { t } = useI18n()
    const full = t('heroMottoPrefix') + '<em>' + t('heroMottoEm') + '</em>,<br />' + t('heroMottoSuffix')
    expect(full).toBe('Game <em>night</em>,<br />made by you.')
  })

  it('composes the hero motto HTML correctly for German', () => {
    setLanguage('de')
    const { t } = useI18n()
    const full = t('heroMottoPrefix') + '<em>' + t('heroMottoEm') + '</em>,<br />' + t('heroMottoSuffix')
    expect(full).toBe('Spiel <em>Nacht</em>,<br />gemacht von dir.')
  })

  it('composes the hero motto HTML correctly for French', () => {
    setLanguage('fr')
    const { t } = useI18n()
    const full = t('heroMottoPrefix') + '<em>' + t('heroMottoEm') + '</em>,<br />' + t('heroMottoSuffix')
    expect(full).toBe('Soirée <em>jeux</em>,<br />faite par vous.')
  })

  it('returns French when browser language is fr', () => {
    setLanguage('fr')
    const { t } = useI18n()
    expect(t('continueButton')).toBe('Continuer →')
  })

  it('falls back to English for unsupported language', () => {
    setLanguage('it')
    const { locale, t } = useI18n()
    expect(locale).toBe('en')
    expect(t('heroSubtitle')).toBe('Type the code your host shared.')
  })

  it('returns key if missing in both locale and English fallback', () => {
    setLanguage('en')
    const { t } = useI18n()
    expect(t('nonexistent')).toBe('nonexistent')
  })
})
