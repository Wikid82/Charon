import { describe, expect, it } from 'vitest'

import { DOCS_SITE_URL, LOGIN_PROTECTION_DOCS_URL, TRUSTED_PROXIES_DOCS_URL } from '../docs'

describe('docs constants', () => {
  it('builds page URLs under the docs site', () => {
    expect(DOCS_SITE_URL).toBe('https://wikid82.github.io/Charon/docs')
    expect(TRUSTED_PROXIES_DOCS_URL).toBe(`${DOCS_SITE_URL}/configuration/trusted-proxies`)
    expect(LOGIN_PROTECTION_DOCS_URL).toBe(`${DOCS_SITE_URL}/features/login-protection`)
  })
})
