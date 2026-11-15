import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys, readLocaleKey } from './utils/localeKeys'

const NEW_KEYS = [
  'errors.forwardTargetRefused',
  'errors.destinationRefused',
  'errors.connectionTestFailed',
  'proxyHosts.notInActiveConfig',
  'proxyHosts.notInActiveConfigHint',
  'remoteServers.testLatency',
] as const

describe('connection test and passive note messages in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for each new key in every locale', () => {
    expect(missingLocaleKeys(NEW_KEYS)).toEqual([])
  })

  it('keeps the {{ms}} placeholder in every locale latency text', () => {
    const missing = Object.entries(localeTrees)
      .filter(([, tree]) => !String(readLocaleKey(tree, 'remoteServers.testLatency')).includes('{{ms}}'))
      .map(([path]) => path)
    expect(missing).toEqual([])
  })

  it('uses the server-matching English texts', () => {
    const en = localeTrees['../../locales/en/translation.json']
    expect(readLocaleKey(en, 'errors.forwardTargetRefused')).toBe('This forward target is not allowed.')
    expect(readLocaleKey(en, 'errors.destinationRefused')).toBe('This destination is not allowed.')
    expect(readLocaleKey(en, 'errors.connectionTestFailed')).toBe('Could not connect to the target.')
    expect(readLocaleKey(en, 'proxyHosts.notInActiveConfig')).toBe('Not in the active configuration')
  })
})
