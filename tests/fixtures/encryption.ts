/**
 * Encryption Management Test Fixtures
 *
 * Shared test data for Encryption Management E2E tests.
 * These fixtures provide consistent test data for testing encryption key
 * rotation, status display, and validation scenarios.
 */

// ============================================================================
// Encryption Status Types
// ============================================================================

/**
 * Encryption status interface matching API response
 */
export interface EncryptionStatus {
  current_version: number;
  next_key_configured: boolean;
  legacy_key_count: number;
  providers_on_current_version: number;
  providers_on_older_versions: number;
}

/**
 * Extended encryption status with additional metadata
 */
export interface EncryptionStatusExtended extends EncryptionStatus {
  last_rotation_at?: string;
  next_rotation_recommended?: string;
  key_algorithm?: string;
  key_size?: number;
}

/**
 * Key rotation result interface
 */
export interface KeyRotationResult {
  success: boolean;
  new_version: number;
  providers_updated: number;
  providers_failed: number;
  message: string;
  timestamp: string;
}

/**
 * Key validation result interface
 */
export interface KeyValidationResult {
  valid: boolean;
  current_key_ok: boolean;
  next_key_ok: boolean;
  legacy_keys_ok: boolean;
  errors?: string[];
}

// ============================================================================
// Healthy Status Fixtures
// ============================================================================

/**
 * Healthy encryption status - all providers on current version
 */
export const healthyEncryptionStatus: EncryptionStatus = {
  current_version: 2,
  next_key_configured: true,
  legacy_key_count: 0,
  providers_on_current_version: 5,
  providers_on_older_versions: 0,
};

/**
 * Healthy encryption status with extended metadata
 */
export const healthyEncryptionStatusExtended: EncryptionStatusExtended = {
  ...healthyEncryptionStatus,
  last_rotation_at: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(), // 30 days ago
  next_rotation_recommended: new Date(Date.now() + 60 * 24 * 60 * 60 * 1000).toISOString(), // 60 days from now
  key_algorithm: 'AES-256-GCM',
  key_size: 256,
};

/**
 * Initial setup status - first key version
 */
export const initialEncryptionStatus: EncryptionStatus = {
  current_version: 1,
  next_key_configured: false,
  legacy_key_count: 0,
  providers_on_current_version: 0,
  providers_on_older_versions: 0,
};

// ============================================================================
// Status Requiring Action Fixtures
// ============================================================================

/**
 * Status indicating rotation is needed - providers on older versions
 */
export const needsRotationStatus: EncryptionStatus = {
  current_version: 1,
  next_key_configured: true,
  legacy_key_count: 1,
  providers_on_current_version: 3,
  providers_on_older_versions: 2,
};

/**
 * Status with many providers needing update
 */
export const manyProvidersOutdatedStatus: EncryptionStatus = {
  current_version: 3,
  next_key_configured: true,
  legacy_key_count: 2,
  providers_on_current_version: 5,
  providers_on_older_versions: 10,
};

/**
 * Status without next key configured
 */
export const noNextKeyStatus: EncryptionStatus = {
  current_version: 2,
  next_key_configured: false,
  legacy_key_count: 0,
  providers_on_current_version: 5,
  providers_on_older_versions: 0,
};

/**
 * Status with legacy keys that should be cleaned up
 */
export const legacyKeysStatus: EncryptionStatus = {
  current_version: 4,
  next_key_configured: true,
  legacy_key_count: 3,
  providers_on_current_version: 8,
  providers_on_older_versions: 0,
};

// ============================================================================
// Key Rotation Result Fixtures
// ============================================================================

/**
 * Successful key rotation result
 */
export const successfulRotationResult: KeyRotationResult = {
  success: true,
  new_version: 3,
  providers_updated: 5,
  providers_failed: 0,
  message: 'Key rotation completed successfully',
  timestamp: new Date().toISOString(),
};

/**
 * Partial success rotation result (some providers failed)
 */
export const partialRotationResult: KeyRotationResult = {
  success: true,
  new_version: 3,
  providers_updated: 4,
  providers_failed: 1,
  message: 'Key rotation completed with 1 provider requiring manual update',
  timestamp: new Date().toISOString(),
};

/**
 * Failed key rotation result
 */
export const failedRotationResult: KeyRotationResult = {
  success: false,
  new_version: 2, // unchanged
  providers_updated: 0,
  providers_failed: 5,
  message: 'Key rotation failed: Unable to decrypt existing credentials',
  timestamp: new Date().toISOString(),
};

// ============================================================================
// Key Validation Result Fixtures
// ============================================================================

/**
 * Successful key validation result
 */
export const validKeyValidationResult: KeyValidationResult = {
  valid: true,
  current_key_ok: true,
  next_key_ok: true,
  legacy_keys_ok: true,
};

/**
 * Validation result with next key not configured
 */
export const noNextKeyValidationResult: KeyValidationResult = {
  valid: true,
  current_key_ok: true,
  next_key_ok: false,
  legacy_keys_ok: true,
  errors: ['Next encryption key is not configured'],
};

/**
 * Validation result with errors
 */
export const invalidKeyValidationResult: KeyValidationResult = {
  valid: false,
  current_key_ok: false,
  next_key_ok: false,
  legacy_keys_ok: false,
  errors: [
    'Current encryption key is invalid or corrupted',
    'Next encryption key is not configured',
    'Legacy key at version 1 cannot be loaded',
  ],
};

/**
 * Validation result with legacy key issues
 */
export const legacyKeyIssuesValidationResult: KeyValidationResult = {
  valid: true,
  current_key_ok: true,
  next_key_ok: true,
  legacy_keys_ok: false,
  errors: ['Legacy key at version 0 is missing - some old credentials may be unrecoverable'],
};

// ============================================================================
// Rotation History Types and Fixtures
// ============================================================================

/**
 * Rotation history entry interface
 */
export interface RotationHistoryEntry {
  id: number;
  from_version: number;
  to_version: number;
  providers_updated: number;
  providers_failed: number;
  initiated_by: string;
  initiated_at: string;
  completed_at: string;
  status: 'completed' | 'partial' | 'failed';
  notes?: string;
}

/**
 * Mock rotation history entries
 */
export const rotationHistory: RotationHistoryEntry[] = [
  {
    id: 3,
    from_version: 1,
    to_version: 2,
    providers_updated: 5,
    providers_failed: 0,
    initiated_by: 'admin@example.com',
    initiated_at: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
    completed_at: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000 + 5000).toISOString(),
    status: 'completed',
  },
  {
    id: 2,
    from_version: 0,
    to_version: 1,
    providers_updated: 3,
    providers_failed: 0,
    initiated_by: 'admin@example.com',
    initiated_at: new Date(Date.now() - 90 * 24 * 60 * 60 * 1000).toISOString(),
    completed_at: new Date(Date.now() - 90 * 24 * 60 * 60 * 1000 + 3000).toISOString(),
    status: 'completed',
  },
  {
    id: 1,
    from_version: 0,
    to_version: 1,
    providers_updated: 2,
    providers_failed: 1,
    initiated_by: 'admin@example.com',
    initiated_at: new Date(Date.now() - 120 * 24 * 60 * 60 * 1000).toISOString(),
    completed_at: new Date(Date.now() - 120 * 24 * 60 * 60 * 1000 + 8000).toISOString(),
    status: 'partial',
    notes: 'One provider had invalid credentials and was skipped',
  },
];

/**
 * Empty rotation history (initial setup)
 */
export const emptyRotationHistory: RotationHistoryEntry[] = [];

// ============================================================================
// UI Status Messages
// ============================================================================

/**
 * Status message configurations for different encryption states
 */
export const statusMessages = {
  healthy: {
    title: 'Encryption Status: Healthy',
    description: 'All credentials are encrypted with the current key version.',
    severity: 'success' as const,
  },
  needsRotation: {
    title: 'Rotation Recommended',
    description: 'Some providers are using older encryption keys.',
    severity: 'warning' as const,
  },
  noNextKey: {
    title: 'Next Key Not Configured',
    description: 'Configure a next key before rotation is needed.',
    severity: 'info' as const,
  },
  criticalIssue: {
    title: 'Encryption Issue Detected',
    description: 'There are issues with the encryption configuration.',
    severity: 'error' as const,
  },
};

// ============================================================================
// API Helper Functions
// ============================================================================

/**
 * Get encryption status via API
 */
export async function getEncryptionStatus(
  request: { get: (url: string) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> }
): Promise<EncryptionStatus> {
  const response = await request.get('/api/v1/admin/encryption/status');

  if (!response.ok()) {
    throw new Error('Failed to get encryption status');
  }

  return response.json() as Promise<EncryptionStatus>;
}

/**
 * Rotate encryption key via API
 */
export async function rotateEncryptionKey(
  request: { post: (url: string, options?: { data?: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> }
): Promise<KeyRotationResult> {
  const response = await request.post('/api/v1/admin/encryption/rotate', {});

  if (!response.ok()) {
    const result = await response.json() as KeyRotationResult;
    return result;
  }

  return response.json() as Promise<KeyRotationResult>;
}

/**
 * Validate encryption keys via API
 */
export async function validateEncryptionKeys(
  request: { post: (url: string, options?: { data?: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> }
): Promise<KeyValidationResult> {
  const response = await request.post('/api/v1/admin/encryption/validate', {});

  return response.json() as Promise<KeyValidationResult>;
}

/**
 * Get rotation history via API
 */
export async function getRotationHistory(
  request: { get: (url: string) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> }
): Promise<RotationHistoryEntry[]> {
  const response = await request.get('/api/v1/admin/encryption/history');

  if (!response.ok()) {
    throw new Error('Failed to get rotation history');
  }

  return response.json() as Promise<RotationHistoryEntry[]>;
}

// ============================================================================
// Test Scenario Helpers
// ============================================================================

/**
 * Generate a status based on provider counts
 */
export function generateEncryptionStatus(
  currentProviders: number,
  outdatedProviders: number,
  version: number = 2
): EncryptionStatus {
  return {
    current_version: version,
    next_key_configured: true,
    legacy_key_count: outdatedProviders > 0 ? 1 : 0,
    providers_on_current_version: currentProviders,
    providers_on_older_versions: outdatedProviders,
  };
}

/**
 * Create a mock rotation history entry
 */
export function createRotationHistoryEntry(
  fromVersion: number,
  toVersion: number,
  success: boolean = true
): RotationHistoryEntry {
  const now = new Date();
  return {
    id: Date.now(),
    from_version: fromVersion,
    to_version: toVersion,
    providers_updated: success ? 5 : 2,
    providers_failed: success ? 0 : 3,
    initiated_by: 'test@example.com',
    initiated_at: now.toISOString(),
    completed_at: new Date(now.getTime() + 5000).toISOString(),
    status: success ? 'completed' : 'partial',
  };
}
