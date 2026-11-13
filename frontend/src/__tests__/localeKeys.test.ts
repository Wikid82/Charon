import { describe, expect, it } from 'vitest'

import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys, readLocaleKey } from './utils/localeKeys'

describe('locale key helpers', () => {
  it('names every locale that lacks a key', () => {
    const missing = missingLocaleKeys(['common.__no_such_key__'])

    expect(missing).toHaveLength(EXPECTED_LOCALE_COUNT)
    expect(missing.every((entry) => entry.endsWith(' common.__no_such_key__'))).toBe(true)
    expect(missing).toContain('../../locales/en/translation.json common.__no_such_key__')
  })

  it('reads nested keys and stops at missing branches', () => {
    const tree = { a: { b: 'text', blank: '  ' } }

    expect(readLocaleKey(tree, 'a.b')).toBe('text')
    expect(readLocaleKey(tree, 'a.b.c')).toBeUndefined()
    expect(readLocaleKey(tree, 'x.y')).toBeUndefined()
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })
})
