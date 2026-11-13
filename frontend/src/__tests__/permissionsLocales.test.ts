import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

const NEW_KEYS = [
  'permissions.adminOnlyError',
  'permissions.hiddenForRole',
  'permissions.dnsAdminOnly',
] as const

describe('role-restricted option text in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each permissions key in every locale', () => {
    expect(missingLocaleKeys(NEW_KEYS)).toEqual([])
  })
})
