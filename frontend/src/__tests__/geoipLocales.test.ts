import { describe, it, expect } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys, readLocaleKey } from './utils/localeKeys'

/** The English bundle is the source of truth: every `security.geoip.*` key is derived from it, so new keys cannot drift. */
const EN_PATH_SUFFIX = '/en/translation.json'
const enEntry = Object.entries(localeTrees).find(([path]) => path.endsWith(EN_PATH_SUFFIX))
if (!enEntry) {
  throw new Error(`No locale file ending in ${EN_PATH_SUFFIX} is bundled`)
}
const enTree = enEntry[1]

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

/** Flattens a nested locale object into dotted leaf keys, for example `source.label`. */
function leafKeys(node: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(node).flatMap(([key, value]) =>
    isRecord(value) ? leafKeys(value, `${prefix}${key}.`) : [`${prefix}${key}`],
  )
}

const enGeoip = readLocaleKey(enTree, 'security.geoip')
if (!isRecord(enGeoip)) {
  throw new Error('The en translation.json has no security.geoip object')
}
const GEOIP_KEYS = leafKeys(enGeoip).map((key) => `security.geoip.${key}`)

const PLACEHOLDER = /\{\{\s*[\w.]+\s*\}\}/g

/** The sorted interpolation placeholders in a value, so translated strings must reuse exactly the same ones. */
function placeholdersOf(value: unknown): string[] {
  return typeof value === 'string' ? [...(value.match(PLACEHOLDER) ?? [])].sort() : []
}

describe('security.geoip strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it('derives a non-empty key list from the English bundle', () => {
    expect(GEOIP_KEYS.length).toBeGreaterThan(0)
  })

  it('has a non-empty value for every security.geoip key in every locale', () => {
    expect(missingLocaleKeys(GEOIP_KEYS)).toEqual([])
  })

  it.each(GEOIP_KEYS)('keeps the placeholders of %s identical to English in every locale', (key) => {
    const expected = placeholdersOf(readLocaleKey(enTree, key))
    const mismatches = Object.entries(localeTrees).flatMap(([path, tree]) => {
      const actual = placeholdersOf(readLocaleKey(tree, key))
      return JSON.stringify(actual) === JSON.stringify(expected) ? [] : [`${path} ${key}`]
    })
    expect(mismatches).toEqual([])
  })
})
