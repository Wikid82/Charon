import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

const NEW_KEYS = [
  'setup.setupCodeLabel',
  'setup.setupCodeHint',
  'setup.errors.setup_code_invalid',
  'setup.errors.rate_limited',
] as const

describe('first-run setup code text in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each setup key in every locale', () => {
    expect(missingLocaleKeys(NEW_KEYS)).toEqual([])
  })
})
