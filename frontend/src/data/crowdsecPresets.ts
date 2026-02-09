export interface CrowdsecPreset {
  slug: string
  title: string
  description: string
  content: string
  tags?: string[]
  warning?: string
}

export const CROWDSEC_PRESETS: CrowdsecPreset[] = [
  {
    slug: 'bot-mitigation-essentials',
    title: 'Bot Mitigation Essentials',
    description:
      'Core HTTP parsers and scenarios aimed at credential stuffing, scanners, and bad crawlers with minimal false positives.',
    tags: ['bots', 'web', 'auth'],
    content: `configs:
  collections:
    - crowdsecurity/base-http-scenarios
    - crowdsecurity/http-cve
    - crowdsecurity/http-bad-user-agent
  parsers:
    - crowdsecurity/http-logs
    - crowdsecurity/nginx-logs
    - crowdsecurity/apache2-logs
  scenarios:
    - crowdsecurity/http-bf
    - crowdsecurity/http-sensitive-files
    - crowdsecurity/http-probing
    - crowdsecurity/http-crawl-non_statics
  postoverflows:
    - crowdsecurity/whitelists
`,
    warning: 'Best for internet-facing apps; ensure allowlists cover SSO and monitoring probes.',
  },
  {
    slug: 'honeypot-friendly-defaults',
    title: 'Honeypot Friendly Defaults',
    description: 'Lightweight defaults tuned for tarpits and research honeypots to reduce noisy bans.',
    tags: ['low-noise', 'ssh', 'http'],
    content: `configs:
  collections:
    - crowdsecurity/sshd
    - crowdsecurity/caddy
  parsers:
    - crowdsecurity/sshd-logs
    - crowdsecurity/caddy-logs
  scenarios:
    - crowdsecurity/ssh-bf
    - crowdsecurity/http-backdoors-attempts
    - crowdsecurity/http-probing
  postoverflows:
    - crowdsecurity/whitelists
`,
    warning: 'Keep honeypot endpoints isolated; avoid applying to production ingress.',
  },
  {
    slug: 'geolocation-aware',
    title: 'Geolocation Aware',
    description: 'Adds geo-enrichment and region-aware scenarios to tighten access by country.',
    tags: ['geo', 'access-control'],
    content: `configs:
  collections:
    - crowdsecurity/geoip-enricher
  scenarios:
    - crowdsecurity/geo-fencing
    - crowdsecurity/geo-bf
  postoverflows:
    - crowdsecurity/whitelists
`,
    warning: 'Requires GeoIP database. Pair with ACLs to avoid blocking legitimate traffic.',
  },
]

export const findCrowdsecPreset = (slug: string): CrowdsecPreset | undefined => {
  return CROWDSEC_PRESETS.find((preset) => preset.slug === slug)
}
