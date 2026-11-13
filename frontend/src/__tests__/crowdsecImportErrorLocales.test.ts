import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

/** Translation keys of the import refusal messages every locale must carry. */
const REFUSAL_KEYS = ['importErrorNotWritable', 'importErrorArchiveTooLarge', 'importErrorRequestTooLarge'] as const

describe('import refusal messages in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each refusal key in every locale', () => {
    expect(missingLocaleKeys(REFUSAL_KEYS.map((key) => `crowdsecConfig.${key}`))).toEqual([])
  })
})
