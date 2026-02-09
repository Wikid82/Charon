# Frontend Test Coverage Improvement Plan

## Objective
Increase frontend test coverage to **88%** locally while maintaining stable CI builds. Current overall line coverage is **84.73%**.

## Strategy

1.  **Target Low Coverage / High Value Areas**: Focus on components with complex logic or API interactions that are currently under-tested.
2.  **Environment-Specific Thresholds**: Implement dynamic coverage thresholds to enforce high standards locally without causing CI fragility.

## Targeted Files

### 1. `src/api/plugins.ts` (Current: 0%)
**Complexity**: LOW
**Value**: MEDIUM (Core API interactions)
**Test Cases**:
- `getPlugins`: Mocks client.get, returns data.
- `getPlugin`: Mocks client.get with ID.
- `enablePlugin`: Mocks client.post with ID.
- `disablePlugin`: Mocks client.post with ID.
- `reloadPlugins`: Mocks client.post, verifies return count.

### 2. `src/components/PermissionsPolicyBuilder.tsx` (Current: ~32%)
**Complexity**: MEDIUM
**Value**: HIGH (Complex string manipulation logic)
**Test Cases**:
- Renders correctly with empty value.
- Parses existing JSON value into state.
- Adds a new feature with `self` allowing.
- Adds a new feature with custom origin.
- Updates existing feature when added again.
- Removes a feature.
- "Quick Add" buttons populate multiple features.
- Generates correct Permissions-Policy header string preview.
- Handles invalid JSON gracefully.

### 3. `src/components/DNSProviderForm.tsx` (Current: ~55%)
**Complexity**: HIGH
**Value**: HIGH (Critical configuration form)
**Test Cases**:
- Renders default state correctly.
- Pre-fills form when editing an existing provider.
- Changes inputs based on selected `Provider Type` (e.g., Cloudflare vs Route53).
- Validates required fields.
- Handles `Test Connection` success/failure states.
- Submits create payload correctly.
- Submits update payload correctly.
- Toggles "Advanced Settings".
- Handles Multi-Credential mode toggles.

### 4. `src/utils/validation.ts` (Current: ~0%)
**Complexity**: LOW
**Value**: HIGH (Security and data validation logic)
**Test Cases**:
- `isValidEmail`: valid emails, invalid emails, empty strings.
- `isIPv4`: valid IPs, invalid IPs, out of range numbers.
- `isPrivateOrDockerIP`:
  - 10.x.x.x (Private)
  - 172.16-31.x.x (Private/Docker)
  - 192.168.x.x (Private)
  - Public IPs (e.g. 8.8.8.8)
- `isLikelyDockerContainerIP`:
  - 172.17-31.x.x (Docker range)
  - Non-docker IPs.

### 5. `src/utils/proxyHostsHelpers.ts` (Current: ~0%)
**Complexity**: MEDIUM
**Value**: MEDIUM (UI Helper logic)
**Test Cases**:
- `formatSettingLabel`: Verify correct labels for keys.
- `settingHelpText`: Verify help text mapping.
- `settingKeyToField`: Verify identity mapping.
- `applyBulkSettingsToHosts`:
  - Applies settings to multiple hosts.
  - Handles missing hosts gracefully.
  - Reports progress callback.
  - Updates error count on failure.

### 6. `src/components/ProxyHostForm.tsx` (Current: ~78% lines, ~61% func)
**Complexity**: VERY HIGH (1378 lines)
**Value**: MAXIMUM (Core Component)
**Test Cases**:
- **Missing Paths Analysis**: Focus on the ~40% of functions not called (likely validation, secondary tabs, dynamic rows).
- **Secondary Tabs**: "Custom Locations", "Advanced" (HSTS, HTTP/2).
- **SSL Flows**: Let's Encrypt vs Custom certificates generation flows.
- **Dynamic Rows**: Adding/removing upstream servers, rewrites interactions.
- **Error Simulation**: API failures during connection testing.

### 7. `src/components/CredentialManager.tsx` (Current: ~50.7%)
**Complexity**: MEDIUM (132 lines)
**Value**: HIGH (Security sensitive)
**Missing Lines**: ~65 lines
**Strategy**:
- Test CRUD operations for different credential types.
- Verify error handling during creation and deletion.
- Test empty states and loading states.

### 8. `src/pages/CrowdSecConfig.tsx` (Current: ~82.5%)
**Complexity**: HIGH (332 lines)
**Value**: MEDIUM (Configuration page)
**Missing Lines**: ~58 lines
**Strategy**:
- Focus on form interactions for all configuration sections.
- Test "Enable/Disable" toggle flows.
- Verify API error handling when saving configuration.

## Configuration Changes

### Dynamic Thresholds
Modify `frontend/vitest.config.ts` to set coverage thresholds based on the environment.

```typescript
const isCI = process.env.CI === 'true';

export default defineConfig({
  // ...
  test: {
    coverage: {
      // ...
      thresholds: {
        lines: isCI ? 83 : 88,
        functions: isCI ? 78 : 88,
        branches: isCI ? 77 : 85,
        statements: isCI ? 83 : 88,
      }
    }
  }
})
```

## Execution Plan

1.  **Implement Tests (Phase 1)**:
    - Create `src/api/__tests__/plugins.test.ts`
    - Create `src/components/__tests__/PermissionsPolicyBuilder.test.tsx`
    - Create `src/components/__tests__/DNSProviderForm.test.tsx` (or expand existing)
2.  **Implement Tests (Phase 2)**:
    - Create `src/utils/__tests__/validation.test.ts`
    - Create `src/utils/__tests__/proxyHostsHelpers.test.ts`
3.  **Implement Tests (Phase 3 - The Heavy Lifter)**:
    - **Target**: `src/components/ProxyHostForm.tsx`
    - **Goal**: >90% coverage for this 1.4k line file.
    - **Strategy**: Expand `src/components/__tests__/ProxyHostForm.test.tsx` to cover edge cases, secondary tabs, and validation logic.
4.  **Implement Tests (Phase 4 - The Final Push)**:
    - **Target**: `src/components/CredentialManager.tsx` and `src/pages/CrowdSecConfig.tsx`
    - **Goal**: Reduce missing lines by >100 (combined).
    - **Strategy**: Create dedicated test files focusing on the unreached branches identified in coverage reports.
5.  **Update Configuration**:
    - Update `frontend/vitest.config.ts`
6.  **Verify**:
    - Run `npm run test:coverage` locally to confirm >88%.
    - Verify CI build simulation.
