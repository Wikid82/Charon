import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

/** Translation keys of the external proxy listen-on setting every locale must carry. */
const LISTEN_ON_KEYS = [
  'bindLabel',
  'bindHint',
  'bindAuto',
  'bindAutoResolved',
  'bindAll',
  'bindCustom',
  'bindCustomLabel',
  'bindCustomPlaceholder',
  'bindAllNote',
  'loopbackNote',
  'fallbackNote',
  'bindListening',
  'bindInvalid',
  'saveError',
  'cardAutomaticNote',
  'bindReconnectNotice',
] as const

describe('external proxy listen-on strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each listen-on key in every locale', () => {
    expect(missingLocaleKeys(LISTEN_ON_KEYS.map((key) => `hecate.externalProxy.${key}`))).toEqual([])
  })
})
