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
    languages.forEach((lang) => {
      expect(i18n.hasResourceBundle(lang, 'translation')).toBe(true)
    })
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
