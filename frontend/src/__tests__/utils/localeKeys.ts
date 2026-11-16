/** Every locale the app bundles. A new locale file must be counted here, so it cannot skip the key checks. */
export const EXPECTED_LOCALE_COUNT = 28

/** A parsed `translation.json`, typed only as far as the key lookups need. */
export type LocaleTree = Record<string, unknown>

/** Every bundled locale file, keyed by its module path. */
export const localeTrees = import.meta.glob('../../locales/*/translation.json', {
  import: 'default',
  eager: true,
}) as Record<string, LocaleTree>

/** Reads a dot-separated key (for example `notificationCenter.alerts.label`) from a locale tree. */
export function readLocaleKey(tree: LocaleTree, dottedKey: string): unknown {
  return dottedKey
    .split('.')
    .reduce<unknown>(
      (node, part) =>
        typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[part] : undefined,
      tree,
    );
}

/**
 * Lists every "<locale file> <key>" pair whose value is missing or blank. An empty result means every listed key
 * is a non-blank string in every bundled locale. Tests assert the result is `[]`, so a failure names each gap.
 */
export function missingLocaleKeys(keys: readonly string[]): string[] {
  return Object.entries(localeTrees).flatMap(([path, tree]) =>
    keys
      .filter((key) => {
        const text = readLocaleKey(tree, key);
        return typeof text !== 'string' || text.trim().length === 0;
      })
      .map((key) => `${path} ${key}`),
  );
}

/** Matches a static key passed as the first argument of `t('...')`. */
const STATIC_T_CALL = /\bt\(\s*'([A-Za-z0-9_.]+)'/g

/** Returns the distinct static translation keys (`t('a.b')`) that a component source passes to `t()`. */
export function usedTranslationKeys(source: string): string[] {
  return [...new Set([...source.matchAll(STATIC_T_CALL)].map((match) => match[1]))]
}
