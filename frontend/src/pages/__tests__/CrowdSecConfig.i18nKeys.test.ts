import { describe, it, expect } from 'vitest'

type Json = { [key: string]: Json | string }

const LOCALES = ['de', 'en', 'es', 'fr', 'zh'] as const
const sourceModules = {
  ...import.meta.glob('../CrowdSecConfig.tsx', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../utils/crowdsecExport.ts', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../components/crowdsec/HubItemsRestorePanel.tsx', { query: '?raw', import: 'default', eager: true }),
  ...import.meta.glob('../../components/crowdsec/DashboardTimeRangeSelector.tsx', { query: '?raw', import: 'default', eager: true }),
} as Record<string, string>
const localeModules = import.meta.glob('../../locales/*/translation.json', { import: 'default', eager: true }) as Record<string, Json>
const NAMESPACES = ['crowdsecConfig', 'security.crowdsec']

const loadLocale = (locale: string): Json => localeModules[`../../locales/${locale}/translation.json`]

const lookup = (tree: Json, path: string): Json | string | undefined =>
  path.split('.').reduce<Json | string | undefined>((node, part) => {
    if (node && typeof node === 'object' && part in node) return node[part]
    return undefined
  }, tree)

const leafKeys = (node: Json | string, prefix: string): string[] =>
  typeof node === 'string'
    ? [prefix]
    : Object.entries(node).flatMap(([k, v]) => leafKeys(v, `${prefix}.${k}`))

const usedKeys = (): string[] => {
  const keys = new Set<string>()
  for (const src of Object.values(sourceModules)) {
    for (const m of src.matchAll(/\bt\(\s*'([A-Za-z0-9_.]+)'/g)) keys.add(m[1])
  }
  return [...keys]
}

// Calls and assignments whose string arguments reach the user, so they must go through t().
const USER_VISIBLE_CALL = /\b(?:toast\.(?:success|error|info|warning)|setPresetStatusMessage|setValidationError|getServerErrorMessage)\(/g
const USER_VISIBLE_LINE = /\b(?:sub)?message:|\bnextErrors\.\w+ =|\breturn '[A-Z]|^\s*[?:] '[A-Z]|\bmsg = .*'[A-Z]/

/** Returns the source of each user-visible call (up to its closing parenthesis) and each matching line. */
const userVisibleCalls = (src: string): string[] => {
  const calls: string[] = []
  for (const m of src.matchAll(USER_VISIBLE_CALL)) {
    let depth = 0
    let end = m.index
    for (; end < src.length; end++) {
      if (src[end] === '(') depth++
      if (src[end] === ')' && --depth === 0) break
    }
    calls.push(src.slice(m.index, end + 1))
  }
  return [...calls, ...src.split('\n').filter((line) => USER_VISIBLE_LINE.test(line))]
}

/** True when the text still holds a quoted or template literal with letters after removing t('key') lookups. */
const hasLiteralText = (code: string): boolean =>
  /(['`])[^'`]*[A-Za-z]{3}[^'`]*\1/.test(code.replace(/\bt\(\s*'[^']*'/g, 't(').replace(/\bt\(\s*[^'\s][^)]*\?[^)]*\)/g, 't('))

describe('CrowdSec config i18n keys', () => {
  it('finds static keys in the sources', () => {
    expect(usedKeys().length).toBeGreaterThan(50)
  })

  it('every static t() key resolves to a string in the en locale', () => {
    const en = loadLocale('en')
    const missing = usedKeys().filter((k) => typeof lookup(en, k) !== 'string')
    expect(missing).toEqual([])
  })

  it.each(LOCALES.filter((l) => l !== 'en'))('%s has the same key set as en for CrowdSec namespaces', (locale) => {
    const en = loadLocale('en')
    const other = loadLocale(locale)
    for (const ns of NAMESPACES) {
      const enNode = lookup(en, ns)
      const otherNode = lookup(other, ns)
      expect(enNode, `en.${ns}`).toBeDefined()
      expect(otherNode, `${locale}.${ns}`).toBeDefined()
      expect(leafKeys(otherNode as Json, ns).sort()).toEqual(leafKeys(enNode as Json, ns).sort())
    }
  })

  it.each(LOCALES)('%s defines the import notice keys and the updated export description', (locale) => {
    const tree = loadLocale(locale)
    for (const key of [
      'crowdsecConfig.importNeedsValues',
      'crowdsecConfig.importNeedsValuesMore',
      'crowdsecConfig.packages.description',
    ]) {
      expect(typeof lookup(tree, key), `${locale}.${key}`).toBe('string')
    }
    expect(lookup(tree, 'crowdsecConfig.importNeedsValues') as string).toContain('{{files}}')
    expect(lookup(tree, 'crowdsecConfig.importNeedsValuesMore') as string).toContain('{{count}}')
  })

  it.each(LOCALES)('%s offers the one-year ban option and no permanent option', (locale) => {
    const tree = loadLocale(locale)
    for (const key of ['crowdsecConfig.banModal.duration365d', 'crowdsecConfig.banModal.durationHelp']) {
      expect(typeof lookup(tree, key), `${locale}.${key}`).toBe('string')
    }
    expect(lookup(tree, 'crowdsecConfig.banModal.durationPermanent')).toBeUndefined()
  })

  it('CrowdSecConfig.tsx has no hard-coded user-visible strings in toasts, status messages or errors', () => {
    const [source] = Object.entries(sourceModules).filter(([path]) => path.endsWith('/CrowdSecConfig.tsx')).map(([, src]) => src)
    expect(source).toBeDefined()
    expect(userVisibleCalls(source).length).toBeGreaterThan(20)
    expect(userVisibleCalls(source).filter(hasLiteralText)).toEqual([])
  })
})
