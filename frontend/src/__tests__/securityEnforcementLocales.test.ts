import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

const NEW_KEYS = ['security.enforcement.acl.geoip_unavailable', 'accessLists.errors.access_list_rules_required'] as const

describe('access-list enforcement messages in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each enforcement key in every locale', () => {
    expect(missingLocaleKeys(NEW_KEYS)).toEqual([])
  })
})
