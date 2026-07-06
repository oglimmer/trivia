import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useI18n } from './useI18n'

describe('useI18n', () => {
  const originalLanguage = navigator.language

  beforeEach(() => {
    Object.defineProperty(navigator, 'language', {
      value: originalLanguage,
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(navigator, 'language', {
      value: originalLanguage,
      writable: true,
      configurable: true,
    })
  })

  it('returns English translations when browser language is en', () => {
    (navigator as any).language = 'en'
    const { t, locale } = useI18n()
    expect(locale).toBe('en')
    expect(t('heroEyebrow')).toBe('Bring-your-own-question')
  })

  it('returns German when browser language is de', () => {
    (navigator as any).language = 'de'
    const { t } = useI18n()
    expect(t('heroMotto')).toBe('Spiel <em>Nacht</em>,<br />gemacht von dir.')
  })

  it('returns French when browser language is fr', () => {
    (navigator as any).language = 'fr'
    const { t } = useI18n()
    expect(t('continueButton')).toBe('Continuer →')
  })

  it('falls back to English for unsupported language', () => {
    (navigator as any).language = 'it'
    const { locale, t } = useI18n()
    expect(locale).toBe('en')
    expect(t('heroSubtitle')).toBe('Type the code your host shared.')
  })

  it('returns key if missing in both locale and English fallback', () => {
    (navigator as any).language = 'en'
    const { t } = useI18n()
    expect(t('nonexistent')).toBe('nonexistent')
  })
})
