import { describe, it, expect } from 'vitest'

import { ALERT_PRESENTATION } from '../constants/alerts'
import { EXPECTED_LOCALE_COUNT, localeTrees, missingLocaleKeys, readLocaleKey } from './utils/localeKeys'

/** Static bell strings, including the ones that were hard-coded before they were translated. */
const BELL_KEYS = [
  'notificationCenter.title',
  'notificationCenter.bellLabel',
  'notificationCenter.markAllRead',
  'notificationCenter.empty',
  'notificationCenter.close',
  'notificationCenter.updateAvailable',
  'notificationCenter.viewChangelog',
  'notificationCenter.alerts.label',
  'notificationCenter.alerts.clearsItself',
] as const;

/** Keys every registered alert needs: its title and body, plus the docs and issue link labels. */
const alertKeysFor = (alertKey: string, titleKey: string): string[] => [
  titleKey,
  `notificationCenter.alerts.${alertKey}.docsLink`,
  `notificationCenter.alerts.${alertKey}.issueHint`,
  `notificationCenter.alerts.${alertKey}.issueLink`,
];

const GEOIP_BODY_KEYS = [
  'notificationCenter.alerts.geoip_unavailable.message',
  'notificationCenter.alerts.geoip_unavailable.messagePendingApply',
] as const;

/** Placeholders each string must keep, so interpolation still fills every value in every language. */
const REQUIRED_PLACEHOLDERS: ReadonlyArray<readonly [string, readonly string[]]> = [
  ['notificationCenter.bellLabel', ['{{count}}']],
  ['notificationCenter.updateAvailable', ['{{version}}']],
  ...GEOIP_BODY_KEYS.map((key): [string, readonly string[]] => [key, ['{{count}}', '{{lists}}']]),
];

describe('notification center strings in every locale', () => {
  it(`bundles exactly ${EXPECTED_LOCALE_COUNT} locale files`, () => {
    expect(Object.keys(localeTrees)).toHaveLength(EXPECTED_LOCALE_COUNT);
  });

  it('has a non-empty value for every static bell string', () => {
    expect(missingLocaleKeys(BELL_KEYS)).toEqual([]);
  });

  it.each(Object.entries(ALERT_PRESENTATION))('has every text for the registered alert "%s"', (alertKey, presentation) => {
    expect(missingLocaleKeys(alertKeysFor(alertKey, presentation.titleKey))).toEqual([]);
  });

  it('has the body variant for each geoip_unavailable message', () => {
    expect(missingLocaleKeys(GEOIP_BODY_KEYS)).toEqual([]);
  });

  it.each(REQUIRED_PLACEHOLDERS.flatMap(([key, placeholders]) => placeholders.map((p) => [key, p] as const)))(
    '%s keeps the %s placeholder in every locale',
    (key, placeholder) => {
      for (const [path, tree] of Object.entries(localeTrees)) {
        const text = readLocaleKey(tree, key);
        expect(typeof text === 'string' && text.includes(placeholder), `${path} ${key}`).toBe(true);
      }
    },
  );
});
