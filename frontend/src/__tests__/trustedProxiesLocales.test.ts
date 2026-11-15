import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys, readLocaleKey } from './utils/localeKeys'

const TRUSTED_PROXY_KEYS = [
  'security.trustedProxies.title',
  'security.trustedProxies.description',
  'security.trustedProxies.fieldLabel',
  'security.trustedProxies.fieldHint',
  'security.trustedProxies.alwaysTrusted',
  'security.trustedProxies.apiLayerNote',
  'security.trustedProxies.apiLayerNone',
  'security.trustedProxies.sourceSetting',
  'security.trustedProxies.sourceEnvironment',
  'security.trustedProxies.sourceDefault',
  'security.trustedProxies.droppedEntries',
  'security.trustedProxies.saved',
  'security.trustedProxies.saveFailed',
  'security.trustedProxies.reset',
  'security.trustedProxies.resetDone',
  'security.trustedProxies.loadError',
  'security.trustedProxies.docsLink',
  'errors.trustedProxyInvalid',
  'errors.trustedProxyTooBroad',
  'errors.trustedProxiesTooMany',
] as const

/** Placeholders a translated string must keep, so an interpolated value still reaches the reader. */
const PLACEHOLDERS: Record<string, string[]> = {
  'security.trustedProxies.alwaysTrusted': ['{{entries}}'],
  'security.trustedProxies.apiLayerNote': ['{{entries}}'],
  'security.trustedProxies.droppedEntries': ['{{entries}}'],
  'errors.trustedProxyInvalid': ['{{entry}}'],
  'errors.trustedProxyTooBroad': ['{{entry}}'],
}

describe('trusted proxy strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('has a non-empty value for every trusted proxy key in every locale', () => {
    expect(missingLocaleKeys(TRUSTED_PROXY_KEYS)).toEqual([])
  })

  it('keeps each interpolation placeholder in every locale', () => {
    const gaps = Object.entries(localeTrees).flatMap(([path, tree]) =>
      Object.entries(PLACEHOLDERS).flatMap(([key, placeholders]) => {
        const text = readLocaleKey(tree, key)
        return typeof text === 'string' && placeholders.every((p) => text.includes(p)) ? [] : [`${path} ${key}`]
      }),
    )
    expect(gaps).toEqual([])
  })
})
