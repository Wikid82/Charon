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
  {
    id: 'known-botnets',
    name: 'Block Known Botnet IPs',
    description: 'Block IPs known to be part of active botnets and malware networks',
    category: 'security',
    type: 'blacklist',
    ipRanges: [
      // Spamhaus DROP list entries (curated subset)
      { cidr: '5.8.10.0/24', description: 'Spamhaus DROP - malware' },
      { cidr: '5.188.206.0/24', description: 'Spamhaus DROP - spam/botnet' },
      { cidr: '23.94.0.0/15', description: 'Known bulletproof hosting' },
      { cidr: '31.13.195.0/24', description: 'Spamhaus EDROP - malware' },
      { cidr: '45.14.224.0/22', description: 'Abuse.ch - malware hosting' },
      { cidr: '77.247.110.0/24', description: 'Known C&C servers' },
      { cidr: '91.200.12.0/22', description: 'Spamhaus DROP - botnet' },
      { cidr: '91.211.116.0/22', description: 'Known spam origin' },
      { cidr: '185.234.216.0/22', description: 'Abuse.ch - botnet infrastructure' },
      { cidr: '194.165.16.0/23', description: 'Known attack infrastructure' },
    ],
    estimatedIPs: '~50,000',
    dataSource: 'Spamhaus DROP/EDROP + abuse.ch',
    dataSourceUrl: 'https://www.spamhaus.org/drop/',
  },
  {
    id: 'cloud-scanners',
    name: 'Block Cloud Scanner IPs',
    description: 'Block IP ranges used by mass scanning services (Shodan, Censys, etc.)',
    category: 'security',
    type: 'blacklist',
    ipRanges: [
      // Shodan scanning IPs
      { cidr: '71.6.135.0/24', description: 'Shodan scanners' },
      { cidr: '71.6.167.0/24', description: 'Shodan scanners' },
      { cidr: '82.221.105.0/24', description: 'Shodan scanners' },
      { cidr: '85.25.43.0/24', description: 'Shodan scanners' },
      { cidr: '85.25.103.0/24', description: 'Shodan scanners' },
      { cidr: '93.120.27.0/24', description: 'Shodan scanners' },
      { cidr: '198.108.66.0/24', description: 'Shodan scanners' },
      { cidr: '198.20.69.0/24', description: 'Shodan scanners' },
      // Censys scanning IPs
      { cidr: '162.142.125.0/24', description: 'Censys scanners' },
      { cidr: '167.248.133.0/24', description: 'Censys scanners' },
      { cidr: '167.94.138.0/24', description: 'Censys scanners' },
      { cidr: '167.94.145.0/24', description: 'Censys scanners' },
      { cidr: '167.94.146.0/24', description: 'Censys scanners' },
      // SecurityTrails/BinaryEdge
      { cidr: '45.33.32.0/24', description: 'Security scanners' },
      { cidr: '45.33.34.0/24', description: 'Security scanners' },
    ],
    estimatedIPs: '~4,000',
    dataSource: 'Shodan/Censys official scanner lists',
    dataSourceUrl: 'https://help.shodan.io/the-basics/what-is-shodan',
    warning: 'Blocks known scanner IPs. New scanners may not be included.',
  },
  {
    id: 'tor-exit-nodes',
    name: 'Block Tor Exit Nodes',
    description: 'Block known Tor network exit nodes to prevent anonymous access',
    category: 'advanced',
    type: 'blacklist',
    ipRanges: [
      // Tor exit node ranges (subset - changes frequently)
      { cidr: '185.220.100.0/22', description: 'Tor exit nodes' },
      { cidr: '185.220.101.0/24', description: 'Tor exit nodes' },
      { cidr: '185.220.102.0/24', description: 'Tor exit nodes' },
      { cidr: '185.100.84.0/22', description: 'Tor exit nodes' },
      { cidr: '185.100.86.0/24', description: 'Tor exit nodes' },
      { cidr: '185.100.87.0/24', description: 'Tor exit nodes' },
      { cidr: '176.10.99.0/24', description: 'Tor exit nodes' },
      { cidr: '176.10.104.0/22', description: 'Tor exit nodes' },
      { cidr: '51.15.0.0/16', description: 'Scaleway - common Tor hosting' },
    ],
    estimatedIPs: '~70,000',
    dataSource: 'Tor Project Exit Node List',
    dataSourceUrl: 'https://check.torproject.org/exit-addresses',
    warning: 'Tor exit nodes change frequently. List may be incomplete.',
  },
  {
    id: 'vpn-datacenter-ips',
    name: 'Block VPN & Datacenter IPs',
    description: 'Block known VPN providers and datacenter IP ranges commonly used for abuse',
    category: 'advanced',
    type: 'blacklist',
    ipRanges: [
      // Common VPN/Datacenter ranges used for abuse
      { cidr: '104.238.128.0/17', description: 'Vultr hosting - common VPN' },
      { cidr: '45.77.0.0/16', description: 'Vultr hosting' },
      { cidr: '66.42.32.0/19', description: 'Choopa/Vultr' },
      { cidr: '149.28.0.0/16', description: 'Vultr Japan/Singapore' },
      { cidr: '155.138.128.0/17', description: 'Vultr hosting' },
      { cidr: '207.148.64.0/18', description: 'Vultr hosting' },
      { cidr: '209.250.224.0/19', description: 'Vultr hosting' },
    ],
    estimatedIPs: '~600,000',
    dataSource: 'Known VPN/Datacenter ranges',
    dataSourceUrl: 'https://www.vultr.com/resources/faq/',
    warning: 'May block legitimate VPN users. Use with caution.',
  },
  {
    id: 'scraper-bots',
    name: 'Block Web Scraper Bots',
    description: 'Block known aggressive web scraping services and bad bots',
    category: 'advanced',
    type: 'blacklist',
    ipRanges: [
      // Aggressive scrapers
      { cidr: '35.192.0.0/12', description: 'GCP - common scraper hosting' },
      { cidr: '54.208.0.0/13', description: 'AWS us-east - scraper hosting' },
      { cidr: '13.32.0.0/12', description: 'AWS CloudFront - may be scrapers' },
      { cidr: '18.188.0.0/14', description: 'AWS us-east-2 - known bots' },
      // Known bad bot operators
      { cidr: '216.244.66.0/24', description: 'DotBot scraper' },
      { cidr: '46.4.122.0/24', description: 'MJ12bot scraper' },
      { cidr: '144.76.38.0/24', description: 'SEMrush bot' },
      { cidr: '46.229.168.0/24', description: 'BLEXBot scraper' },
    ],
    estimatedIPs: '~4 million',
    dataSource: 'Bad bot IP feeds',
    dataSourceUrl: 'https://radar.cloudflare.com/traffic/bots',
    warning: 'Blocks large cloud ranges. May impact legitimate services.',
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
