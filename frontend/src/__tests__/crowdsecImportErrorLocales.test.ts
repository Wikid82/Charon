import { describe, it, expect } from 'vitest'

/** Shape of a locale file as far as this test reads it: the CrowdSec section of the translations. */
type LocaleFile = { crowdsecConfig?: Record<string, unknown> }

/** Every locale the app bundles. A new locale file must be counted here, so it cannot skip the key check. */
const EXPECTED_LOCALE_COUNT = 28

/** Translation keys of the import refusal messages every locale must carry. */
const REFUSAL_KEYS = ['importErrorNotWritable', 'importErrorArchiveTooLarge', 'importErrorRequestTooLarge'] as const

const localeFiles = import.meta.glob('../locales/*/translation.json', { import: 'default', eager: true }) as Record<
  string,
  LocaleFile
>

describe('import refusal messages in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeFiles)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it.each(
    Object.entries(localeFiles).flatMap(([path, tree]) => REFUSAL_KEYS.map((key) => [path, tree, key] as const)),
  )('%s has a non-empty crowdsecConfig.%s', (_path, tree, key) => {
    const text: unknown = tree.crowdsecConfig?.[key]
    expect(typeof text === 'string' && text.trim().length > 0).toBe(true)
  })
})
