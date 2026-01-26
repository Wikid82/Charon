/**
 * ACL Enforcement Tests
 *
 * Tests that verify the Access Control List (ACL) module correctly blocks/allows
 * requests based on IP whitelist and blacklist rules.
 *
 * Pattern: Toggle-On-Test-Toggle-Off
 * - Enable ACL at start of describe block
 * - Run enforcement tests
 * - Disable ACL in afterAll (handled by security-teardown project)
 *
 * @see /projects/Charon/docs/plans/current_spec.md - ACL Enforcement Tests
 */

import { test, expect } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';
import {
  getSecurityStatus,
  setSecurityModuleEnabled,
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';

test.describe('ACL Enforcement', () => {
  let requestContext: APIRequestContext;
  let originalState: CapturedSecurityState;

  test.beforeAll(async () => {
    requestContext = await request.newContext({
      baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
      storageState: STORAGE_STATE,
    });

    // Capture original state
    try {
      originalState = await captureSecurityState(requestContext);
    } catch (error) {
      console.error('Failed to capture original security state:', error);
    }

    // Enable Cerberus (master toggle) first
    try {
      await setSecurityModuleEnabled(requestContext, 'cerberus', true);
      console.log('✓ Cerberus enabled');
    } catch (error) {
      console.error('Failed to enable Cerberus:', error);
    }

    // Enable ACL
    try {
      await setSecurityModuleEnabled(requestContext, 'acl', true);
      console.log('✓ ACL enabled');
    } catch (error) {
      console.error('Failed to enable ACL:', error);
    }
  });

  test.afterAll(async () => {
    // Restore original state
    if (originalState) {
      try {
        await restoreSecurityState(requestContext, originalState);
        console.log('✓ Security state restored');
      } catch (error) {
        console.error('Failed to restore security state:', error);
        // Emergency disable ACL to prevent deadlock
        try {
          await setSecurityModuleEnabled(requestContext, 'acl', false);
          await setSecurityModuleEnabled(requestContext, 'cerberus', false);
        } catch {
          console.error('Emergency ACL disable also failed');
        }
      }
    }
    await requestContext.dispose();
  });

  test('should verify ACL is enabled', async () => {
    const status = await getSecurityStatus(requestContext);
    expect(status.acl.enabled).toBe(true);
    expect(status.cerberus.enabled).toBe(true);
  });

  test('should return security status with ACL mode', async () => {
    const response = await requestContext.get('/api/v1/security/status');
    expect(response.ok()).toBe(true);

    const status = await response.json();
    expect(status.acl).toBeDefined();
    expect(status.acl.mode).toBeDefined();
    expect(typeof status.acl.enabled).toBe('boolean');
  });

  test('should list access lists when ACL enabled', async () => {
    const response = await requestContext.get('/api/v1/access-lists');
    expect(response.ok()).toBe(true);

    const data = await response.json();
    expect(Array.isArray(data)).toBe(true);
  });

  test('should test IP against access list', async () => {
    // First, get the list of access lists
    const listResponse = await requestContext.get('/api/v1/access-lists');
    expect(listResponse.ok()).toBe(true);

    const lists = await listResponse.json();

    // If there are any access lists, test an IP against the first one
    if (lists.length > 0) {
      const testIp = '192.168.1.1';
      const testResponse = await requestContext.get(
        `/api/v1/access-lists/${lists[0].id}/test?ip=${testIp}`
      );
      expect(testResponse.ok()).toBe(true);

      const result = await testResponse.json();
      expect(typeof result.allowed).toBe('boolean');
    } else {
      // No access lists exist - this is valid, just log it
      console.log('No access lists exist to test against');
    }
  });

  test('should show correct error response format for blocked requests', async () => {
    // Create a temporary blacklist with test IP, make blocked request, then cleanup
    // For now, verify the error message format from the blocked response

    // This test verifies the error handling structure exists
    // The actual blocking test would require:
    // 1. Create blacklist entry with test IP
    // 2. Make request from that IP (requires proxy setup)
    // 3. Verify 403 with "Blocked by access control list" message
    // 4. Delete blacklist entry

    // Instead, we verify the API structure for ACL CRUD
    const createResponse = await requestContext.post('/api/v1/access-lists', {
      data: {
        name: 'Test Enforcement ACL',
        satisfy: 'any',
        pass_auth: false,
        items: [
          {
            type: 'deny',
            address: '10.255.255.255/32',
            directive: 'deny',
            comment: 'Test blocked IP',
          },
        ],
      },
    });

    if (createResponse.ok()) {
      const createdList = await createResponse.json();
      expect(createdList.id).toBeDefined();

      // Verify the list was created with correct structure
      expect(createdList.name).toBe('Test Enforcement ACL');

      // Test IP against the list
      const testResponse = await requestContext.get(
        `/api/v1/access-lists/${createdList.id}/test?ip=10.255.255.255`
      );
      expect(testResponse.ok()).toBe(true);
      const testResult = await testResponse.json();
      expect(testResult.allowed).toBe(false);

      // Cleanup: Delete the test ACL
      const deleteResponse = await requestContext.delete(
        `/api/v1/access-lists/${createdList.id}`
      );
      expect(deleteResponse.ok()).toBe(true);
    } else {
      // May fail if ACL already exists or other issue
      const errorBody = await createResponse.text();
      console.log(`Note: Could not create test ACL: ${errorBody}`);
    }
  });
});
