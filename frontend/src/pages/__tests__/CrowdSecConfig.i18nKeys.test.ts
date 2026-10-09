import { describe, it, expect } from 'vitest'

type Json = { [key: string]: Json | string }

const LOCALES = ['de', 'en', 'es', 'fr', 'zh'] as const
const sourceModules = {
  ...import.meta.glob('../CrowdSecConfig.tsx', { query: '?raw', import: 'default', eager: true }),
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
})
