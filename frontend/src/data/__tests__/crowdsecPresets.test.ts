import { describe, it, expect } from 'vitest'

import { CROWDSEC_PRESETS } from '../crowdsecPresets'

describe('crowdsecPresets', () => {
  it('lists only the curated presets served by the backend', () => {
    expect(CROWDSEC_PRESETS.map((p) => p.slug)).toEqual(['honeypot-friendly-defaults', 'geoip-enrichment'])
  })

  it('does not retain the removed geolocation-aware slug or client-side content', () => {
    for (const preset of CROWDSEC_PRESETS) {
      expect(preset.slug).not.toBe('geolocation-aware')
      expect(preset).not.toHaveProperty('content')
    }
  })

  it('has well-formed metadata for each preset', () => {
    for (const preset of CROWDSEC_PRESETS) {
      expect(preset.slug).toMatch(/^[a-z0-9]+(-[a-z0-9]+)*$/)
      expect(preset.title).toMatch(/^[A-Z]/)
      expect(preset.description.split(' ').length).toBeGreaterThan(5)
      expect(preset.tags?.length).toBeGreaterThan(0)
      for (const tag of preset.tags ?? []) {
        expect(tag).toMatch(/^[a-z0-9-]+$/)
      }
      expect(preset.warning).toMatch(/[.!]$/)
    }
  })

  it('describes GeoIP enrichment as enrichment only', () => {
    const geo = CROWDSEC_PRESETS.find((p) => p.slug === 'geoip-enrichment')
    expect(geo?.title).toBe('GeoIP Enrichment')
    expect(geo?.description).toBe(
      'Enriches CrowdSec log events with GeoIP data (country and ASN). Useful for decision context and dashboards.',
    )
    expect(geo?.warning).toBe('Enrichment only: this does not block traffic by region. Use access lists for region rules.')
    expect(geo?.tags).toEqual(['geo', 'enrichment'])
  })

  it('makes no unsupported low-noise or tarpit claims for the honeypot preset', () => {
    const honeypot = CROWDSEC_PRESETS.find((p) => p.slug === 'honeypot-friendly-defaults')
    expect(honeypot?.tags).not.toContain('low-noise')
    expect(`${honeypot?.description} ${honeypot?.warning}`).not.toMatch(/low-noise|tarpit|reduce noisy/i)
    expect(honeypot?.description).toContain('whitelists parser')
  })
})
