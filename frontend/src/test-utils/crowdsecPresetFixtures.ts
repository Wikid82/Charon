export interface HubPresetFixture {
  slug: string
  title: string
  description: string
  content: string
}

/** Hub-sourced preset fixtures (previews come from the hub pull, not the frontend). */
export const HUB_PRESET_FIXTURES: HubPresetFixture[] = [
  {
    slug: 'bot-mitigation-essentials',
    title: 'Bot Mitigation Essentials',
    description: 'Core HTTP parsers and scenarios aimed at credential stuffing, scanners, and bad crawlers.',
    content: `configs:
  collections:
    - crowdsecurity/base-http-scenarios
    - crowdsecurity/http-cve
  parsers:
    - crowdsecurity/http-logs
`,
  },
  {
    slug: 'second-hub-preset',
    title: 'Second Hub Preset',
    description: 'A second hub preset used to exercise preset selection.',
    content: `configs:
  scenarios:
    - crowdsecurity/ssh-bf
`,
  },
]
