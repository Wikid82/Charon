import { describe, it, expect } from 'vitest'

/** Shape of a locale file as far as this test reads it: the CrowdSec section of the translations. */
type LocaleFile = { crowdsecConfig?: Record<string, unknown> }

/** Every locale the app bundles. A new locale file must be counted here, so it cannot skip the key check. */
const EXPECTED_LOCALE_COUNT = 28

/** Translation key of the message shown when an import cannot change the configuration folder. */
const NOT_WRITABLE_KEY = 'importErrorNotWritable'

const localeFiles = import.meta.glob('../locales/*/translation.json', { import: 'default', eager: true }) as Record<
  string,
  LocaleFile
>

describe('import_config_not_writable message in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeFiles)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it.each(Object.entries(localeFiles))('%s has a non-empty crowdsecConfig.importErrorNotWritable', (_path, tree) => {
    const text: unknown = tree.crowdsecConfig?.[NOT_WRITABLE_KEY]
    expect(typeof text === 'string' && text.trim().length > 0).toBe(true)
  })
})
