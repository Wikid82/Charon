/**
 * Security Presets for Access Control Lists
 *
 * Data sources:
 * - High-risk countries: Based on common attack origin statistics from threat intelligence feeds
 * - Cloud scanner IPs: Known IP ranges used for mass scanning (Shodan, Censys, etc.)
 * - Botnet IPs: Curated from public blocklists (Spamhaus, abuse.ch, etc.)
 *
 * References:
 * - SANS Internet Storm Center: https://isc.sans.edu/
 * - Spamhaus DROP/EDROP lists: https://www.spamhaus.org/drop/
 * - Abuse.ch threat feeds: https://abuse.ch/
 */

export interface SecurityPreset {
  id: string;
  name: string;
  description: string;
  category: 'security' | 'advanced';
  type: 'geo_blacklist' | 'blacklist';
  countryCodes?: string[];
  ipRanges?: Array<{ cidr: string; description: string }>;
  estimatedIPs: string;
  dataSource: string;
  dataSourceUrl: string;
  warning?: string;
}

export const SECURITY_PRESETS: SecurityPreset[] = [
  {
    id: 'high-risk-countries',
    name: 'Block High-Risk Countries',
    description: 'Block countries with highest attack/spam rates (OFAC sanctioned + known attack sources)',
    category: 'security',
    type: 'geo_blacklist',
    countryCodes: [
      'RU', // Russia
      'CN', // China
      'KP', // North Korea
      'IR', // Iran
      'BY', // Belarus
      'SY', // Syria
      'VE', // Venezuela
      'CU', // Cuba
      'SD', // Sudan
    ],
    estimatedIPs: '~800 million',
    dataSource: 'SANS ISC Top Attack Origins',
    dataSourceUrl: 'https://isc.sans.edu/sources.html',
    warning: 'This blocks entire countries. Legitimate users from these countries will be blocked.',
  },
  {
    id: 'expanded-threat-countries',
    name: 'Block Expanded Threat List',
    description: 'High-risk countries plus additional sources of bot traffic and spam',
    category: 'security',
    type: 'geo_blacklist',
    countryCodes: [
      'RU', // Russia
      'CN', // China
      'KP', // North Korea
      'IR', // Iran
      'BY', // Belarus
      'SY', // Syria
      'VE', // Venezuela
      'CU', // Cuba
      'SD', // Sudan
      'PK', // Pakistan
      'BD', // Bangladesh
      'NG', // Nigeria
      'UA', // Ukraine (high bot activity)
      'VN', // Vietnam
      'ID', // Indonesia
    ],
    estimatedIPs: '~1.2 billion',
    dataSource: 'Combined threat intelligence feeds',
    dataSourceUrl: 'https://isc.sans.edu/',
    warning: 'Aggressive blocking. May impact legitimate international users.',
  },
];

export const getPresetById = (id: string): SecurityPreset | undefined => {
  return SECURITY_PRESETS.find((preset) => preset.id === id);
};

export const getPresetsByCategory = (category: 'security' | 'advanced'): SecurityPreset[] => {
  return SECURITY_PRESETS.filter((preset) => preset.category === category);
};

/**
 * Calculate approximate number of IPs in a CIDR range
 */
export const calculateCIDRSize = (cidr: string): number => {
  const parts = cidr.split('/');
  if (parts.length !== 2) return 1;

  const bits = parseInt(parts[1], 10);
  if (isNaN(bits) || bits < 0 || bits > 32) return 1;

  return Math.pow(2, 32 - bits);
};

/**
 * Format IP count for display
 */
export const formatIPCount = (count: number): string => {
  if (count >= 1000000000) {
    return `${(count / 1000000000).toFixed(1)}B`;
  }
  if (count >= 1000000) {
    return `${(count / 1000000).toFixed(1)}M`;
  }
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}K`;
  }
  return count.toString();
};

/**
 * Calculate total IPs in a list of CIDR ranges
 */
export const calculateTotalIPs = (cidrs: string[]): number => {
  return cidrs.reduce((total, cidr) => total + calculateCIDRSize(cidr), 0);
};
