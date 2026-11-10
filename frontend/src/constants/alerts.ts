import { GEOIP_TROUBLESHOOTING_DOCS_URL } from './docs';

import type { ActiveAlert } from '../api/system';

/**
 * How the notification bell presents one kind of active alert. Keys are the server's alert keys.
 * Labels for the docs and issue links are derived from the alert key (`notificationCenter.alerts.<key>.docsLink`,
 * `.issueHint`, `.issueLink`), so every registered key must provide them in every locale.
 */
export interface AlertPresentation {
  /** i18n key of the alert headline. */
  titleKey: string;
  /** Picks the i18n key of the body text from the alert's own fields. */
  messageKey: (alert: ActiveAlert) => string;
  /** Documentation page the alert links to. */
  docsUrl: string;
  /** Whether to ask the user to report the problem as a GitHub issue. */
  showIssueLink: boolean;
}

const GEOIP_UNAVAILABLE_PREFIX = 'notificationCenter.alerts.geoip_unavailable';

export const ALERT_PRESENTATION: Readonly<Record<string, AlertPresentation>> = {
  geoip_unavailable: {
    titleKey: `${GEOIP_UNAVAILABLE_PREFIX}.title`,
    messageKey: (alert) =>
      alert.database_available === true
        ? `${GEOIP_UNAVAILABLE_PREFIX}.messagePendingApply`
        : `${GEOIP_UNAVAILABLE_PREFIX}.message`,
    docsUrl: GEOIP_TROUBLESHOOTING_DOCS_URL,
    showIssueLink: true,
  },
};

/** Returns the presentation for a server alert key, or undefined when the key is not registered. */
export function getAlertPresentation(key: string): AlertPresentation | undefined {
  return Object.prototype.hasOwnProperty.call(ALERT_PRESENTATION, key)
    ? ALERT_PRESENTATION[key]
    : undefined;
}

/**
 * Keeps only the alerts this frontend knows how to show. Unknown keys are dropped silently so a newer
 * backend cannot put unexplained entries in the bell or inflate its count.
 */
export function filterKnownAlerts(alerts: readonly ActiveAlert[]): ActiveAlert[] {
  return alerts.filter((alert) => getAlertPresentation(alert.key) !== undefined);
}
