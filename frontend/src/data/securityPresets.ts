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
    description: 'Block countries with highest attack/spam rates',
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
    description: 'Includes high-risk countries plus additional threat sources',
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
      'UA', // Ukraine (unfortunately high bot activity)
      'VN', // Vietnam
      'ID', // Indonesia
    ],
    estimatedIPs: '~1.2 billion',
    dataSource: 'Combined threat intelligence feeds',
    dataSourceUrl: 'https://isc.sans.edu/',
    warning: 'This is aggressive blocking. May impact legitimate international users.',
  },
  {
    id: 'cloud-scanners',
    name: 'Block Cloud Scanner IPs',
    description: 'Block IP ranges used by mass scanning services',
    category: 'advanced',
    type: 'blacklist',
    ipRanges: [
      // Shodan scanning IPs (examples - real implementation should use current list)
      { cidr: '71.6.135.0/24', description: 'Shodan scanners' },
      { cidr: '71.6.167.0/24', description: 'Shodan scanners' },
      { cidr: '82.221.105.0/24', description: 'Shodan scanners' },
      { cidr: '85.25.43.0/24', description: 'Shodan scanners' },
      { cidr: '85.25.103.0/24', description: 'Shodan scanners' },
      { cidr: '93.120.27.0/24', description: 'Shodan scanners' },
      { cidr: '162.142.125.0/24', description: 'Censys scanners' },
      { cidr: '167.248.133.0/24', description: 'Censys scanners' },
      { cidr: '198.108.66.0/24', description: 'Shodan scanners' },
      { cidr: '198.20.69.0/24', description: 'Shodan scanners' },
    ],
    estimatedIPs: '~3,000',
    dataSource: 'Shodan/Censys official scanner lists',
    dataSourceUrl: 'https://help.shodan.io/the-basics/what-is-shodan',
    warning: 'Only blocks known scanner IPs. New scanner IPs may not be included.',
  },
  {
    id: 'tor-exit-nodes',
    name: 'Block Tor Exit Nodes',
    description: 'Block known Tor network exit nodes',
    category: 'advanced',
    type: 'blacklist',
    ipRanges: [
      // Note: Tor exit nodes change frequently
      // Real implementation should fetch from https://check.torproject.org/exit-addresses
      { cidr: '185.220.100.0/22', description: 'Tor exit nodes' },
      { cidr: '185.220.101.0/24', description: 'Tor exit nodes' },
      { cidr: '185.220.102.0/24', description: 'Tor exit nodes' },
      { cidr: '185.100.84.0/22', description: 'Tor exit nodes' },
      { cidr: '185.100.86.0/24', description: 'Tor exit nodes' },
      { cidr: '185.100.87.0/24', description: 'Tor exit nodes' },
    ],
    estimatedIPs: '~1,200 (changes daily)',
    dataSource: 'Tor Project Exit Node List',
    dataSourceUrl: 'https://check.torproject.org/exit-addresses',
    warning: 'Tor exit nodes change frequently. Consider using a dynamic blocklist service.',
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
