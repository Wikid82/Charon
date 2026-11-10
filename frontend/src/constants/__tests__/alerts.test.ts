import { describe, expect, it } from 'vitest'

import { ALERT_PRESENTATION, filterKnownAlerts, getAlertPresentation } from '../alerts'
import { GEOIP_TROUBLESHOOTING_DOCS_URL } from '../docs'

import type { ActiveAlert } from '../../api/system'

const geoip: ActiveAlert = {
  key: 'geoip_unavailable',
  severity: 'error',
  affected_lists: ['No RU/CN'],
  affected_lists_total: 1,
  database_available: false,
}

describe('ALERT_PRESENTATION', () => {
  it('registers geoip_unavailable with its docs page and issue link', () => {
    const presentation = ALERT_PRESENTATION.geoip_unavailable
    expect(presentation.titleKey).toBe('notificationCenter.alerts.geoip_unavailable.title')
    expect(presentation.docsUrl).toBe(GEOIP_TROUBLESHOOTING_DOCS_URL)
    expect(presentation.showIssueLink).toBe(true)
  })

  it('uses the plain message when no database is available', () => {
    expect(ALERT_PRESENTATION.geoip_unavailable.messageKey(geoip)).toBe(
      'notificationCenter.alerts.geoip_unavailable.message',
    )
    expect(
      ALERT_PRESENTATION.geoip_unavailable.messageKey({ ...geoip, database_available: undefined }),
    ).toBe('notificationCenter.alerts.geoip_unavailable.message')
  })

  it('uses the pending-apply message when a database is available again', () => {
    expect(
      ALERT_PRESENTATION.geoip_unavailable.messageKey({ ...geoip, database_available: true }),
    ).toBe('notificationCenter.alerts.geoip_unavailable.messagePendingApply')
  })
})

describe('getAlertPresentation', () => {
  it('returns the presentation for a registered key', () => {
    expect(getAlertPresentation('geoip_unavailable')).toBe(ALERT_PRESENTATION.geoip_unavailable)
  })

  it('returns undefined for an unknown key', () => {
    expect(getAlertPresentation('future_alert')).toBeUndefined()
  })

  it.each(['constructor', 'toString', '__proto__', 'hasOwnProperty'])(
    'does not resolve inherited property %s',
    (key) => {
      expect(getAlertPresentation(key)).toBeUndefined()
    },
  )
})

describe('filterKnownAlerts', () => {
  it('keeps registered alerts in their order and drops unknown keys', () => {
    const unknown: ActiveAlert = { key: 'future_alert', severity: 'warning' }
    expect(filterKnownAlerts([unknown, geoip])).toEqual([geoip])
  })

  it('returns an empty array for an empty input', () => {
    expect(filterKnownAlerts([])).toEqual([])
  })
})
