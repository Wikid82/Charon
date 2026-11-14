import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

/** Translation keys of the Rotate key flow every locale must carry. */
const ROTATE_KEY_KEYS = [
  'rotateLabel',
  'rotateTitle',
  'rotateDescription',
  'rotateConfirm',
  'rotateSuccessTitle',
  'rotateSuccessDescription',
  'rotateDone',
  'rotateError',
] as const

describe('agent key rotation strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each rotate-key key in every locale', () => {
    expect(missingLocaleKeys(ROTATE_KEY_KEYS.map((key) => `hecate.agentManager.${key}`))).toEqual([])
  })
})
