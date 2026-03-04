/**
 * Certificate Test Fixtures
 *
 * Mock data for SSL Certificate E2E tests.
 * Provides various certificate configurations for testing CRUD operations,
 * ACME challenges, and validation scenarios.
 *
 * @example
 * ```typescript
 * import { letsEncryptCertificate, customCertificateMock, expiredCertificate } from './fixtures/certificates';
 *
 * test('upload custom certificate', async ({ testData }) => {
 *   const { id } = await testData.createCertificate(customCertificateMock);
 * });
 * ```
 */

import { generateDomain, generateUniqueId } from './test-data';

/**
 * Certificate type
 */
export type CertificateType = 'letsencrypt' | 'custom' | 'self-signed';

/**
 * ACME challenge type
 */
export type ChallengeType = 'http-01' | 'dns-01';

/**
 * Certificate status
 */
export type CertificateStatus =
  | 'pending'
  | 'valid'
  | 'expired'
  | 'revoked'
  | 'error';

/**
 * Certificate configuration interface
 */
export interface CertificateConfig {
  /** Domains covered by the certificate */
  domains: string[];
  /** Certificate type */
  type: CertificateType;
  /** PEM-encoded certificate (for custom certs) */
  certificate?: string;
  /** PEM-encoded private key (for custom certs) */
  privateKey?: string;
  /** PEM-encoded intermediate certificates */
  intermediates?: string;
  /** DNS provider ID (for dns-01 challenge) */
  dnsProviderId?: string;
  /** Force renewal even if not expiring */
  forceRenewal?: boolean;
  /** ACME email for notifications */
  acmeEmail?: string;
}

/**
 * Self-signed test certificate and key
 * Valid for testing purposes only - DO NOT use in production
 */
export const selfSignedTestCert = {
  certificate: `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAJC1HiIAZAiUMA0GCSqGSIb3Qw0BBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMjQwMTAxMDAwMDAwWhcNMjkwMTAxMDAwMDAwWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAzUCFQIzxZqSU5LNHJ3m1R8fU3VpMfmTc1DJfKSBnBH4HKvC2vN7T9N9P
test-certificate-data-placeholder-for-testing-purposes-only
-----END CERTIFICATE-----`,
  privateKey: `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDNQIVAjPFmpJTk
s0cnebVHx9TdWkx+ZNzUMl8pIGcEfgcq8La83tP030/0
test-private-key-data-placeholder-for-testing-purposes-only
-----END PRIVATE KEY-----`,
};

/**
 * Let's Encrypt certificate mock
 * Simulates a certificate obtained via ACME
 */
export const letsEncryptCertificate: CertificateConfig = {
  domains: ['app.example.com'],
  type: 'letsencrypt',
  acmeEmail: 'admin@example.com',
};

/**
 * Let's Encrypt certificate with multiple domains (SAN)
 */
export const multiDomainLetsEncrypt: CertificateConfig = {
  domains: ['app.example.com', 'www.example.com', 'api.example.com'],
  type: 'letsencrypt',
  acmeEmail: 'admin@example.com',
};

/**
 * Wildcard certificate mock
 * Uses DNS-01 challenge (required for wildcards)
 */
export const wildcardCertificate: CertificateConfig = {
  domains: ['*.example.com', 'example.com'],
  type: 'letsencrypt',
  acmeEmail: 'admin@example.com',
  dnsProviderId: '', // Will be set dynamically in tests
};

/**
 * Custom certificate mock
 * Uses self-signed certificate for testing
 */
export const customCertificateMock: CertificateConfig = {
  domains: ['custom.example.com'],
  type: 'custom',
  certificate: selfSignedTestCert.certificate,
  privateKey: selfSignedTestCert.privateKey,
};

/**
 * Custom certificate with intermediate chain
 */
export const customCertWithChain: CertificateConfig = {
  domains: ['chain.example.com'],
  type: 'custom',
  certificate: selfSignedTestCert.certificate,
  privateKey: selfSignedTestCert.privateKey,
  intermediates: `-----BEGIN CERTIFICATE-----
MIIDrzCCApegAwIBAgIQCDvgVpBCRrGhdWrJWZHHSjANBgkqhkiG9w0BAQUFADBh
intermediate-certificate-placeholder-for-testing
-----END CERTIFICATE-----`,
};

/**
 * Expired certificate mock
 * For testing expiration handling
 */
export const expiredCertificate = {
  domains: ['expired.example.com'],
  type: 'custom' as CertificateType,
  status: 'expired' as CertificateStatus,
  expiresAt: new Date(Date.now() - 86400000).toISOString(), // Yesterday
  certificate: `-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAJC1HiIAZAiUMA0GCSqGSIb3Qw0BBQUAMEUxCzAJBgNV
expired-certificate-placeholder
-----END CERTIFICATE-----`,
  privateKey: selfSignedTestCert.privateKey,
};

/**
 * Certificate expiring soon
 * For testing renewal warnings
 */
export const expiringCertificate = {
  domains: ['expiring.example.com'],
  type: 'letsencrypt' as CertificateType,
  status: 'valid' as CertificateStatus,
  expiresAt: new Date(Date.now() + 7 * 86400000).toISOString(), // 7 days from now
};

/**
 * Revoked certificate mock
 */
export const revokedCertificate = {
  domains: ['revoked.example.com'],
  type: 'letsencrypt' as CertificateType,
  status: 'revoked' as CertificateStatus,
  revokedAt: new Date().toISOString(),
  revocationReason: 'Key compromise',
};

/**
 * Invalid certificate configurations for validation testing
 */
export const invalidCertificates = {
  /** Empty domains */
  emptyDomains: {
    domains: [],
    type: 'letsencrypt' as CertificateType,
  },

  /** Invalid domain format */
  invalidDomain: {
    domains: ['not a valid domain!'],
    type: 'letsencrypt' as CertificateType,
  },

  /** Missing certificate for custom type */
  missingCertificate: {
    domains: ['custom.example.com'],
    type: 'custom' as CertificateType,
    privateKey: selfSignedTestCert.privateKey,
  },

  /** Missing private key for custom type */
  missingPrivateKey: {
    domains: ['custom.example.com'],
    type: 'custom' as CertificateType,
    certificate: selfSignedTestCert.certificate,
  },

  /** Invalid certificate PEM format */
  invalidCertificatePEM: {
    domains: ['custom.example.com'],
    type: 'custom' as CertificateType,
    certificate: 'not a valid PEM certificate',
    privateKey: selfSignedTestCert.privateKey,
  },

  /** Invalid private key PEM format */
  invalidPrivateKeyPEM: {
    domains: ['custom.example.com'],
    type: 'custom' as CertificateType,
    certificate: selfSignedTestCert.certificate,
    privateKey: 'not a valid PEM private key',
  },

  /** Mismatched certificate and key */
  mismatchedCertKey: {
    domains: ['custom.example.com'],
    type: 'custom' as CertificateType,
    certificate: selfSignedTestCert.certificate,
    privateKey: `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDifferent-key
-----END PRIVATE KEY-----`,
  },

  /** Wildcard without DNS provider */
  wildcardWithoutDNS: {
    domains: ['*.example.com'],
    type: 'letsencrypt' as CertificateType,
    // dnsProviderId is missing - required for wildcards
  },

  /** Too many domains */
  tooManyDomains: {
    domains: Array.from({ length: 150 }, (_, i) => `domain${i}.example.com`),
    type: 'letsencrypt' as CertificateType,
  },

  /** XSS in domain */
  xssInDomain: {
    domains: ['<script>alert(1)</script>.example.com'],
    type: 'letsencrypt' as CertificateType,
  },

  /** Invalid ACME email */
  invalidAcmeEmail: {
    domains: ['app.example.com'],
    type: 'letsencrypt' as CertificateType,
    acmeEmail: 'not-an-email',
  },
};

/**
 * Generate a unique certificate configuration
 * @param overrides - Optional configuration overrides
 * @returns CertificateConfig with unique domain
 *
 * @example
 * ```typescript
 * const cert = generateCertificate({ type: 'custom' });
 * ```
 */
export function generateCertificate(
  overrides: Partial<CertificateConfig> = {}
): CertificateConfig {
  const baseCert: CertificateConfig = {
    domains: [generateDomain('cert')],
    type: 'letsencrypt',
    acmeEmail: `admin-${generateUniqueId()}@test.local`,
    ...overrides,
  };

  // Add certificate/key for custom type
  if (baseCert.type === 'custom' && !baseCert.certificate) {
    baseCert.certificate = selfSignedTestCert.certificate;
    baseCert.privateKey = selfSignedTestCert.privateKey;
  }

  return baseCert;
}

/**
 * Generate wildcard certificate configuration
 * @param baseDomain - Base domain for the wildcard
 * @param dnsProviderId - DNS provider ID for DNS-01 challenge
 * @returns CertificateConfig for wildcard
 */
export function generateWildcardCertificate(
  baseDomain?: string,
  dnsProviderId?: string
): CertificateConfig {
  const domain = baseDomain || `${generateUniqueId()}.test.local`;
  return {
    domains: [`*.${domain}`, domain],
    type: 'letsencrypt',
    acmeEmail: `admin-${generateUniqueId()}@test.local`,
    dnsProviderId,
  };
}

/**
 * Generate multiple unique certificates
 * @param count - Number of certificates to generate
 * @param overrides - Optional configuration overrides
 * @returns Array of CertificateConfig
 */
export function generateCertificates(
  count: number,
  overrides: Partial<CertificateConfig> = {}
): CertificateConfig[] {
  return Array.from({ length: count }, () => generateCertificate(overrides));
}

/**
 * Expected API response for certificate creation
 */
export interface CertificateAPIResponse {
  id: string;
  domains: string[];
  type: CertificateType;
  status: CertificateStatus;
  issuer?: string;
  expires_at?: string;
  issued_at?: string;
  acme_email?: string;
  dns_provider_id?: string;
  created_at: string;
  updated_at: string;
}

/**
 * Mock API response for testing
 */
export function mockCertificateResponse(
  config: Partial<CertificateConfig> = {}
): CertificateAPIResponse {
  const id = generateUniqueId();
  const domains = config.domains || [generateDomain('cert')];
  const isLetsEncrypt = config.type !== 'custom';

  return {
    id,
    domains,
    type: config.type || 'letsencrypt',
    status: 'valid',
    issuer: isLetsEncrypt ? "Let's Encrypt Authority X3" : 'Self-Signed',
    expires_at: new Date(Date.now() + 90 * 86400000).toISOString(), // 90 days
    issued_at: new Date().toISOString(),
    acme_email: config.acmeEmail,
    dns_provider_id: config.dnsProviderId,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}

/**
 * Mock ACME challenge data
 */
export interface ACMEChallengeData {
  type: ChallengeType;
  token: string;
  keyAuthorization: string;
  domain: string;
  status: 'pending' | 'processing' | 'valid' | 'invalid';
}

/**
 * Generate mock ACME HTTP-01 challenge
 */
export function mockHTTP01Challenge(domain: string): ACMEChallengeData {
  return {
    type: 'http-01',
    token: `mock-token-${generateUniqueId()}`,
    keyAuthorization: `mock-auth-${generateUniqueId()}`,
    domain,
    status: 'pending',
  };
}

/**
 * Generate mock ACME DNS-01 challenge
 */
export function mockDNS01Challenge(domain: string): ACMEChallengeData {
  return {
    type: 'dns-01',
    token: `mock-token-${generateUniqueId()}`,
    keyAuthorization: `mock-dns-auth-${generateUniqueId()}`,
    domain: `_acme-challenge.${domain}`,
    status: 'pending',
  };
}
