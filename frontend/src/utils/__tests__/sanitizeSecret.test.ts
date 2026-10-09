import { describe, it, expect } from 'vitest'

import { sanitizeSecret } from '../sanitizeSecret'

describe('sanitizeSecret', () => {
  it('does not mask ordinary words', () => {
    const msg = 'enrollment failed: authentication required for Console registration'
    expect(sanitizeSecret(msg)).toBe(msg)
  })

  it('masks mixed alphanumeric tokens', () => {
    expect(sanitizeSecret('bad key clxyz0123456789abcdef here')).toBe('bad key *** here')
  })

  it('masks hex-like tokens', () => {
    expect(sanitizeSecret('token 3f9a1c7e5b2d4a60 rejected')).toBe('token *** rejected')
  })

  it('masks very long letter-only strings', () => {
    const secret = 'abcdefghijklmnopqrstuvwxyzABCDEFGH'
    expect(sanitizeSecret(`key ${secret}`)).toBe('key ***')
  })

  it('leaves short strings and numbers alone', () => {
    expect(sanitizeSecret('error 404 on port 8080')).toBe('error 404 on port 8080')
  })
})
