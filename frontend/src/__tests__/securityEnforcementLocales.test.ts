import { describe, it, expect } from 'vitest'

/** Shape of a locale file as far as this test reads it. */
type LocaleFile = {
  security?: { enforcement?: { acl?: Record<string, unknown> } }
  accessLists?: { errors?: Record<string, unknown> }
}

/** Every locale the app bundles. A new locale file must be counted here, so it cannot skip the key checks. */
const EXPECTED_LOCALE_COUNT = 28

const localeFiles = import.meta.glob('../locales/*/translation.json', { import: 'default', eager: true }) as Record<
  string,
  LocaleFile
>

const NEW_KEYS: ReadonlyArray<readonly [string, (tree: LocaleFile) => unknown]> = [
  ['security.enforcement.acl.geoip_unavailable', (tree) => tree.security?.enforcement?.acl?.geoip_unavailable],
  ['accessLists.errors.access_list_rules_required', (tree) => tree.accessLists?.errors?.access_list_rules_required],
]

describe('access-list enforcement messages in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeFiles)).toHaveLength(EXPECTED_LOCALE_COUNT)
  })

  it.each(
    Object.entries(localeFiles).flatMap(([path, tree]) => NEW_KEYS.map(([name, read]) => [path, tree, name, read] as const)),
  )('%s has a non-empty %s', (_path, tree, _name, read) => {
    const text = read(tree)
    expect(typeof text === 'string' && text.trim().length > 0).toBe(true)
  })
})
