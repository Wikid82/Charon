import client from './client'

import type { DNSProvider } from './dnsProviders'

/** DNS provider detection result */
export interface DetectionResult {
  domain: string
  detected: boolean
  provider_type?: string
  nameservers: string[]
  confidence: 'high' | 'medium' | 'low' | 'none'
  suggested_provider?: DNSProvider
  error?: string
}

/** Nameserver pattern used for detection */
export interface NameserverPattern {
  pattern: string
  provider_type: string
}

/**
 * Detects DNS provider for a domain by analyzing nameservers.
 * @param domain - Domain name to detect provider for
 * @returns Promise resolving to detection result
 * @throws {AxiosError} If the request fails
 */
export async function detectDNSProvider(domain: string): Promise<DetectionResult> {
  const response = await client.post<DetectionResult>('/dns-providers/detect', { domain })
  return response.data
}

/**
 * Fetches built-in nameserver patterns used for detection.
 * @returns Promise resolving to array of patterns
 * @throws {AxiosError} If the request fails
 */
export async function getDetectionPatterns(): Promise<NameserverPattern[]> {
  const response = await client.get<{ patterns: NameserverPattern[] }>('/dns-providers/patterns')
  return response.data.patterns
}
