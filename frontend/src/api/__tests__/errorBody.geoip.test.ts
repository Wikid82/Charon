import { describe, expect, it } from 'vitest'

import { GEOIP_ERROR_CODES, geoipErrorKey } from '../errorBody'

describe('geoipErrorKey', () => {
  it.each(GEOIP_ERROR_CODES)('maps %s to its own locale key', (code) => {
    expect(geoipErrorKey(code)).toBe(`security.geoip.errors.${code}`)
  })

  it('falls back to the generic text for an unknown code', () => {
    expect(geoipErrorKey('not_a_geoip_code')).toBe('security.geoip.errors.generic')
  })

  it('falls back to the generic text when the code is missing', () => {
    expect(geoipErrorKey(undefined)).toBe('security.geoip.errors.generic')
  })
})
