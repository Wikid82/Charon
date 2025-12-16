import { describe, it, expect } from 'vitest'
import { CROWDSEC_PRESETS, findCrowdsecPreset, type CrowdsecPreset } from '../crowdsecPresets'

describe('crowdsecPresets', () => {
  describe('CROWDSEC_PRESETS', () => {
    it('should contain all expected presets', () => {
      expect(CROWDSEC_PRESETS).toHaveLength(3)
      expect(CROWDSEC_PRESETS.map((p) => p.slug)).toEqual([
        'bot-mitigation-essentials',
        'honeypot-friendly-defaults',
        'geolocation-aware',
      ])
    })

    it('should have valid YAML content for each preset', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.content).toContain('configs:')
        expect(preset.content).toMatch(/collections:|parsers:|scenarios:|postoverflows:/)
      })
    })

    it('should have required metadata fields', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset).toHaveProperty('slug')
        expect(preset).toHaveProperty('title')
        expect(preset).toHaveProperty('description')
        expect(preset).toHaveProperty('content')
        expect(preset.slug).toMatch(/^[a-z0-9-]+$/) // Slug format validation
      })
    })

    it('should have descriptive titles and descriptions', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.title.length).toBeGreaterThan(5)
        expect(preset.description.length).toBeGreaterThan(10)
      })
    })

    it('should have tags for each preset', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.tags).toBeDefined()
        expect(Array.isArray(preset.tags)).toBe(true)
        expect(preset.tags!.length).toBeGreaterThan(0)
      })
    })

    it('should have warnings for production-critical presets', () => {
      const botMitigation = CROWDSEC_PRESETS.find((p) => p.slug === 'bot-mitigation-essentials')
      expect(botMitigation?.warning).toBeDefined()
      expect(botMitigation?.warning).toContain('allowlist')

      const honeypot = CROWDSEC_PRESETS.find((p) => p.slug === 'honeypot-friendly-defaults')
      expect(honeypot?.warning).toBeDefined()
      expect(honeypot?.warning).toContain('honeypot')

      const geo = CROWDSEC_PRESETS.find((p) => p.slug === 'geolocation-aware')
      expect(geo?.warning).toBeDefined()
      expect(geo?.warning).toContain('GeoIP')
    })
  })

  describe('preset content integrity', () => {
    it('should have valid CrowdSec YAML structure', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        const lines = preset.content.split('\n')
        expect(lines[0]).toMatch(/^configs:/)
      })
    })

    it('should reference valid CrowdSec hub items', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        // Extract collection references
        const collections = preset.content.match(/- crowdsecurity\/[\w-]+/g) || []
        collections.forEach((item) => {
          // Hub items can contain underscores (e.g., http-crawl-non_statics)
          expect(item).toMatch(/^- crowdsecurity\/[a-z0-9-_]+$/)
        })
      })
    })

    it('should have proper YAML indentation', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        const lines = preset.content.split('\n')
        // Check that collection/parser/scenario items are indented with spaces
        const itemLines = lines.filter((line) => line.trim().startsWith('- crowdsecurity/'))
        itemLines.forEach((line) => {
          expect(line).toMatch(/^\s{4,}- crowdsecurity\//)
        })
      })
    })

    it('should reference known CrowdSec collections', () => {
      const botMitigation = CROWDSEC_PRESETS.find((p) => p.slug === 'bot-mitigation-essentials')
      expect(botMitigation?.content).toContain('crowdsecurity/base-http-scenarios')
      expect(botMitigation?.content).toContain('crowdsecurity/http-cve')
    })

    it('should reference known CrowdSec parsers', () => {
      const botMitigation = CROWDSEC_PRESETS.find((p) => p.slug === 'bot-mitigation-essentials')
      expect(botMitigation?.content).toContain('crowdsecurity/http-logs')
      expect(botMitigation?.content).toContain('crowdsecurity/nginx-logs')
    })

    it('should reference known CrowdSec scenarios', () => {
      const botMitigation = CROWDSEC_PRESETS.find((p) => p.slug === 'bot-mitigation-essentials')
      expect(botMitigation?.content).toContain('crowdsecurity/http-bf')
      expect(botMitigation?.content).toContain('crowdsecurity/http-probing')
    })

    it('should have whitelists postoverflow for production presets', () => {
      const botMitigation = CROWDSEC_PRESETS.find((p) => p.slug === 'bot-mitigation-essentials')
      expect(botMitigation?.content).toContain('postoverflows:')
      expect(botMitigation?.content).toContain('crowdsecurity/whitelists')
    })
  })

  describe('findCrowdsecPreset', () => {
    it('should find preset by slug', () => {
      const preset = findCrowdsecPreset('bot-mitigation-essentials')
      expect(preset).toBeDefined()
      expect(preset?.slug).toBe('bot-mitigation-essentials')
      expect(preset?.title).toBe('Bot Mitigation Essentials')
    })

    it('should find honeypot preset', () => {
      const preset = findCrowdsecPreset('honeypot-friendly-defaults')
      expect(preset).toBeDefined()
      expect(preset?.slug).toBe('honeypot-friendly-defaults')
      expect(preset?.tags).toContain('low-noise')
    })

    it('should find geolocation preset', () => {
      const preset = findCrowdsecPreset('geolocation-aware')
      expect(preset).toBeDefined()
      expect(preset?.slug).toBe('geolocation-aware')
      expect(preset?.tags).toContain('geo')
    })

    it('should return undefined for non-existent slug', () => {
      const preset = findCrowdsecPreset('non-existent-preset')
      expect(preset).toBeUndefined()
    })

    it('should be case-sensitive', () => {
      const preset = findCrowdsecPreset('BOT-MITIGATION-ESSENTIALS')
      expect(preset).toBeUndefined()
    })

    it('should not match partial slugs', () => {
      const preset = findCrowdsecPreset('bot-mitigation')
      expect(preset).toBeUndefined()
    })

    it('should handle empty string', () => {
      const preset = findCrowdsecPreset('')
      expect(preset).toBeUndefined()
    })

    it('should handle slugs with special characters', () => {
      const preset = findCrowdsecPreset('bot/mitigation')
      expect(preset).toBeUndefined()
    })
  })

  describe('preset-specific validations', () => {
    it('bot-mitigation-essentials should target web threats', () => {
      const preset = findCrowdsecPreset('bot-mitigation-essentials')
      expect(preset?.tags).toContain('bots')
      expect(preset?.tags).toContain('web')
      expect(preset?.tags).toContain('auth')
      expect(preset?.description).toContain('credential stuffing')
      expect(preset?.description).toContain('scanners')
    })

    it('honeypot-friendly-defaults should be low-noise', () => {
      const preset = findCrowdsecPreset('honeypot-friendly-defaults')
      expect(preset?.tags).toContain('low-noise')
      expect(preset?.tags).toContain('ssh')
      expect(preset?.description).toContain('honeypot')
      expect(preset?.content).toContain('crowdsecurity/sshd')
    })

    it('geolocation-aware should require GeoIP', () => {
      const preset = findCrowdsecPreset('geolocation-aware')
      expect(preset?.tags).toContain('geo')
      expect(preset?.tags).toContain('access-control')
      expect(preset?.warning).toContain('GeoIP database')
      expect(preset?.content).toContain('geoip-enricher')
    })
  })

  describe('preset tag consistency', () => {
    it('should have consistent tag naming (lowercase, hyphenated)', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        preset.tags?.forEach((tag) => {
          expect(tag).toMatch(/^[a-z0-9-]+$/)
        })
      })
    })

    it('should have descriptive tags', () => {
      const allTags = CROWDSEC_PRESETS.flatMap((p) => p.tags || [])
      expect(allTags.length).toBeGreaterThan(5)
      expect(new Set(allTags).size).toBeGreaterThan(3) // Multiple unique tags
    })
  })

  describe('slug format validation', () => {
    it('should use lowercase slugs', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.slug).toBe(preset.slug.toLowerCase())
      })
    })

    it('should use hyphens as separators', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.slug).not.toContain('_')
        expect(preset.slug).not.toContain(' ')
      })
    })

    it('should not have leading or trailing hyphens', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.slug).not.toMatch(/^-/)
        expect(preset.slug).not.toMatch(/-$/)
      })
    })

    it('should not have consecutive hyphens', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.slug).not.toContain('--')
      })
    })
  })

  describe('content safety', () => {
    it('should not contain executable code', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.content).not.toContain('<script')
        expect(preset.content).not.toContain('eval(')
        expect(preset.content).not.toContain('exec(')
      })
    })

    it('should not contain SQL injection attempts', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.content).not.toContain('DROP TABLE')
        expect(preset.content).not.toContain('DELETE FROM')
      })
    })

    it('should not contain path traversal attempts', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.content).not.toContain('../')
        expect(preset.content).not.toContain('..\\')
      })
    })
  })

  describe('TypeScript type safety', () => {
    it('should satisfy CrowdsecPreset interface', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        const typedPreset: CrowdsecPreset = preset
        expect(typedPreset.slug).toBeTruthy()
        expect(typedPreset.title).toBeTruthy()
        expect(typedPreset.description).toBeTruthy()
        expect(typedPreset.content).toBeTruthy()
      })
    })

    it('should have optional tags and warning properties', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        if (preset.tags !== undefined) {
          expect(Array.isArray(preset.tags)).toBe(true)
        }
        if (preset.warning !== undefined) {
          expect(typeof preset.warning).toBe('string')
        }
      })
    })
  })

  describe('usability validations', () => {
    it('should have human-readable titles', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.title).not.toMatch(/^[a-z-]+$/) // Not just slug format
        expect(preset.title).toMatch(/^[A-Z]/) // Starts with capital
      })
    })

    it('should have actionable descriptions', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        expect(preset.description.split(' ').length).toBeGreaterThan(5)
      })
    })

    it('should have clear warnings when present', () => {
      CROWDSEC_PRESETS.forEach((preset) => {
        if (preset.warning) {
          expect(preset.warning.length).toBeGreaterThan(10)
          expect(preset.warning).toMatch(/[.!]$/) // Ends with punctuation
        }
      })
    })
  })
})
