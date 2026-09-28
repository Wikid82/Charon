import { describe, it, expect, beforeEach } from 'vitest'

import i18n from '../i18n'

describe('i18n configuration', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en')
  })

  it('initializes with default language', () => {
    expect(i18n.language).toBeDefined()
    expect(i18n.isInitialized).toBe(true)
  })

  it('has all required language resources', () => {
    const languages = ['en', 'es', 'fr', 'de', 'zh']
    for (const lang of languages) {
      expect(i18n.hasResourceBundle(lang, 'translation')).toBe(true)
    }
  })

  it('translates common keys', () => {
    expect(i18n.t('common.save')).toBe('Save')
    expect(i18n.t('common.cancel')).toBe('Cancel')
    expect(i18n.t('common.delete')).toBe('Delete')
  })

  it('translates navigation keys', () => {
    expect(i18n.t('navigation.dashboard')).toBe('Dashboard')
    expect(i18n.t('navigation.settings')).toBe('Settings')
  })

  it('changes language and translates correctly', async () => {
    await i18n.changeLanguage('es')
    expect(i18n.t('common.save')).toBe('Guardar')
    expect(i18n.t('common.cancel')).toBe('Cancelar')

    await i18n.changeLanguage('fr')
    expect(i18n.t('common.save')).toBe('Enregistrer')
    expect(i18n.t('common.cancel')).toBe('Annuler')

    await i18n.changeLanguage('de')
    expect(i18n.t('common.save')).toBe('Speichern')
    expect(i18n.t('common.cancel')).toBe('Abbrechen')

    await i18n.changeLanguage('zh')
    expect(i18n.t('common.save')).toBe('保存')
    expect(i18n.t('common.cancel')).toBe('取消')
  })

  it('falls back to English for missing translations', async () => {
    await i18n.changeLanguage('en')
    const key = 'nonexistent.key'
    expect(i18n.t(key)).toBe(key) // Should return the key itself
  })

  it('supports interpolation', () => {
    expect(i18n.t('dashboard.activeHosts', { count: 5 })).toBe('5 active')
  })
})

describe('locale parity', () => {
  const LOCALES = ['en', 'de', 'es', 'fr', 'zh'] as const

  const flatten = (value: unknown, prefix = ''): string[] => {
    if (typeof value !== 'object' || value === null) return [prefix]
    return Object.entries(value).flatMap(([key, child]) =>
      flatten(child, prefix ? `${prefix}.${key}` : key)
    )
  }

  const keysFor = (locale: string, root: string): string[] =>
    flatten(i18n.getResourceBundle(locale, 'translation')[root], root).sort()

  // Sign-in throttle strings are added in every locale in the same change
  it.each(['errors', 'auth.rateLimitAdminHint', 'auth.rateLimitAdminHintLink', 'security.loginProtection'])(
    'has the same %s keys in every locale',
    (root) => {
      const read = (locale: string) =>
        root.includes('.')
          ? flatten(
              root.split('.').reduce<unknown>((node, part) => (node as Record<string, unknown>)[part], i18n.getResourceBundle(locale, 'translation')),
              root
            ).sort()
          : keysFor(locale, root)
      const expected = read('en')
      expect(expected.length).toBeGreaterThan(0)
      for (const locale of LOCALES) {
        expect(read(locale)).toEqual(expected)
      }
    }
  )

  it.each([
    'errors.tooManyRequestsSeconds',
    'errors.tooManyRequestsMinutes',
    'security.loginProtection.untrustedPrivate',
    'security.loginProtection.untrustedPublic',
  ])('carries %s as bare key plus _one/_other in every locale', (key) => {
    for (const locale of LOCALES) {
      for (const suffix of ['', '_one', '_other']) {
        expect(i18n.exists(`${key}${suffix}`, { lng: locale, fallbackLng: false })).toBe(true)
      }
    }
  })

  it('pluralizes the wait message per locale', async () => {
    await i18n.changeLanguage('en')
    expect(i18n.t('errors.tooManyRequestsSeconds', { count: 1 })).toContain('1 second and')
    expect(i18n.t('errors.tooManyRequestsSeconds', { count: 30 })).toContain('30 seconds')
    expect(i18n.t('errors.tooManyRequestsMinutes', { count: 2 })).toContain('2 minutes')
  })
})
