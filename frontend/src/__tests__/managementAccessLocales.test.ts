import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys } from './utils/localeKeys'

const MANAGEMENT_ACCESS_KEYS = [
  'accessLists.managementAccess.title',
  'accessLists.managementAccess.description',
  'accessLists.managementAccess.fieldsetLabel',
  'accessLists.managementAccess.noLists',
  'accessLists.managementAccess.notRestricted',
  'accessLists.managementAccess.notRestrictedHint',
  'accessLists.managementAccess.restricted',
  'accessLists.managementAccess.notApplied',
  'accessLists.managementAccess.saved',
  'accessLists.managementAccess.saveFailed',
  'accessLists.managementAccess.loadError',
  'errors.managementListUnknown',
  'errors.managementAccessWouldLockOut',
  'errors.accessListInvalid',
] as const

describe('management access strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for every management access key in every locale', () => {
    expect(missingLocaleKeys(MANAGEMENT_ACCESS_KEYS)).toEqual([])
  })
})
