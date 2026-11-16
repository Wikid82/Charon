import { describe, it, expect } from 'vitest'

import { missingLocaleKeys, usedTranslationKeys } from '../../__tests__/utils/localeKeys'

const pageSource = Object.values(
  import.meta.glob('../RateLimiting.tsx', { query: '?raw', import: 'default', eager: true }) as Record<string, string>,
)[0]

describe('RateLimiting page i18n keys', () => {
  const keys = usedTranslationKeys(pageSource)

  it('finds the static keys used by the page', () => {
    expect(keys).toEqual(expect.arrayContaining(['rateLimiting.saveConfiguration', 'rateLimiting.settingUpdated', 'common.failedToUpdate']))
  })

  it('every key the page uses is a non-blank string in all 28 locales', () => {
    expect(missingLocaleKeys(keys)).toEqual([])
  })
})
