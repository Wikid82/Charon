/**
 * Display metadata for curated (Charon-owned) CrowdSec presets.
 *
 * The backend is the single source of truth for what a curated preset installs
 * and applies it server-side; this file only supplies description and warning
 * copy to the UI and must match the backend preset definitions.
 */
export interface CrowdsecPreset {
  slug: string
  title: string
  description: string
  tags?: string[]
  warning?: string
}

export const CROWDSEC_PRESETS: CrowdsecPreset[] = [
  {
    slug: 'honeypot-friendly-defaults',
    title: 'Honeypot Friendly Defaults',
    description:
      'Installs SSH and Caddy log parsing with brute-force and web probing detection, plus the CrowdSec whitelists parser.',
    tags: ['ssh', 'http'],
    warning: 'Applies detection scenarios that can ban offenders; review decisions before using on production ingress.',
  },
  {
    slug: 'geoip-enrichment',
    title: 'GeoIP Enrichment',
    description:
      'Enriches CrowdSec log events with GeoIP data (country and ASN). Useful for decision context and dashboards.',
    tags: ['geo', 'enrichment'],
    warning: 'Enrichment only: this does not block traffic by region. Use access lists for region rules.',
  },
]
