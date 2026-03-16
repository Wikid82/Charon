import { describe, it, expect } from 'vitest';

import {
  SECURITY_PRESETS,
  getPresetById,
  getPresetsByCategory,
  calculateCIDRSize,
  formatIPCount,
  calculateTotalIPs,
} from '../securityPresets';

describe('securityPresets', () => {
  describe('SECURITY_PRESETS', () => {
    it('contains expected presets', () => {
      expect(SECURITY_PRESETS.length).toBeGreaterThan(0);

      // Verify preset structure
      for (const preset of SECURITY_PRESETS) {
        expect(preset).toHaveProperty('id');
        expect(preset).toHaveProperty('name');
        expect(preset).toHaveProperty('description');
        expect(preset).toHaveProperty('category');
        expect(preset).toHaveProperty('type');
        expect(preset).toHaveProperty('estimatedIPs');
        expect(preset).toHaveProperty('dataSource');
        expect(preset).toHaveProperty('dataSourceUrl');
      }
    });

    it('has valid categories', () => {
      const validCategories = ['security', 'advanced'];
      for (const preset of SECURITY_PRESETS) {
        expect(validCategories).toContain(preset.category);
      }
    });

    it('has valid types', () => {
      const validTypes = ['geo_blacklist', 'blacklist'];
      for (const preset of SECURITY_PRESETS) {
        expect(validTypes).toContain(preset.type);
      }
    });

    it('geo_blacklist presets have countryCodes', () => {
      const geoPresets = SECURITY_PRESETS.filter((p) => p.type === 'geo_blacklist');
      for (const preset of geoPresets) {
        expect(preset.countryCodes).toBeDefined();
        expect(preset.countryCodes!.length).toBeGreaterThan(0);
      }
    });

    it('no IP-based blacklist presets are included (CrowdSec handles dynamic IP threats)', () => {
      const ipPresets = SECURITY_PRESETS.filter((p) => p.type === 'blacklist');
      // IP-based blacklists are deprecated and should be handled by CrowdSec / WAF / rate limiting
      expect(ipPresets.length).toBe(0);
    });
  });

  describe('getPresetById', () => {
    it('returns preset when found', () => {
      const preset = getPresetById('high-risk-countries');
      expect(preset).toBeDefined();
      expect(preset?.id).toBe('high-risk-countries');
      expect(preset?.name).toBe('Block High-Risk Countries');
    });

    it('returns undefined when not found', () => {
      const preset = getPresetById('nonexistent-preset');
      expect(preset).toBeUndefined();
    });
  });

  describe('getPresetsByCategory', () => {
    it('returns security category presets', () => {
      const securityPresets = getPresetsByCategory('security');
      expect(securityPresets.length).toBeGreaterThan(0);
      for (const preset of securityPresets) {
        expect(preset.category).toBe('security');
      }
    });

    it('returns advanced category presets (may be empty)', () => {
      const advancedPresets = getPresetsByCategory('advanced');
      expect(Array.isArray(advancedPresets)).toBe(true);
      for (const preset of advancedPresets) {
        expect(preset.category).toBe('advanced');
      }
    });
  });

  describe('calculateCIDRSize', () => {
    it('calculates /32 as 1 IP', () => {
      expect(calculateCIDRSize('192.168.1.1/32')).toBe(1);
    });

    it('calculates /24 as 256 IPs', () => {
      expect(calculateCIDRSize('192.168.1.0/24')).toBe(256);
    });

    it('calculates /16 as 65536 IPs', () => {
      expect(calculateCIDRSize('192.168.0.0/16')).toBe(65536);
    });

    it('calculates /8 as 16777216 IPs', () => {
      expect(calculateCIDRSize('10.0.0.0/8')).toBe(16777216);
    });

    it('calculates /0 as all IPs', () => {
      expect(calculateCIDRSize('0.0.0.0/0')).toBe(4294967296);
    });

    it('returns 1 for single IP without CIDR notation', () => {
      expect(calculateCIDRSize('192.168.1.1')).toBe(1);
    });

    it('returns 1 for invalid CIDR', () => {
      expect(calculateCIDRSize('invalid')).toBe(1);
      expect(calculateCIDRSize('192.168.1.0/abc')).toBe(1);
      expect(calculateCIDRSize('192.168.1.0/-1')).toBe(1);
      expect(calculateCIDRSize('192.168.1.0/33')).toBe(1);
    });
  });

  describe('formatIPCount', () => {
    it('formats small numbers as-is', () => {
      expect(formatIPCount(0)).toBe('0');
      expect(formatIPCount(1)).toBe('1');
      expect(formatIPCount(999)).toBe('999');
    });

    it('formats thousands with K suffix', () => {
      expect(formatIPCount(1000)).toBe('1.0K');
      expect(formatIPCount(1500)).toBe('1.5K');
      expect(formatIPCount(999999)).toBe('1000.0K');
    });

    it('formats millions with M suffix', () => {
      expect(formatIPCount(1000000)).toBe('1.0M');
      expect(formatIPCount(2500000)).toBe('2.5M');
      expect(formatIPCount(999999999)).toBe('1000.0M');
    });

    it('formats billions with B suffix', () => {
      expect(formatIPCount(1000000000)).toBe('1.0B');
      expect(formatIPCount(4294967296)).toBe('4.3B');
    });
  });

  describe('calculateTotalIPs', () => {
    it('calculates total for single CIDR', () => {
      expect(calculateTotalIPs(['192.168.1.0/24'])).toBe(256);
    });

    it('calculates total for multiple CIDRs', () => {
      expect(calculateTotalIPs(['192.168.1.0/24', '10.0.0.0/24'])).toBe(512);
    });

    it('handles empty array', () => {
      expect(calculateTotalIPs([])).toBe(0);
    });

    it('handles mixed valid and invalid CIDRs', () => {
      expect(calculateTotalIPs(['192.168.1.0/24', 'invalid'])).toBe(257);
    });
  });
});
