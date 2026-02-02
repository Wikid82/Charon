---
description: 'Playwright test generation instructions'
applyTo: '**'
---

## Test Writing Guidelines

### Code Quality Standards
- **Locators**: Prioritize user-facing, role-based locators (`getByRole`, `getByLabel`, `getByText`, etc.) for resilience and accessibility. Use `test.step()` to group interactions and improve test readability and reporting.
- **Assertions**: Use auto-retrying web-first assertions. These assertions start with the `await` keyword (e.g., `await expect(locator).toHaveText()`). Avoid `expect(locator).toBeVisible()` unless specifically testing for visibility changes.
- **Timeouts**: Rely on Playwright's built-in auto-waiting mechanisms. Avoid hard-coded waits or increased default timeouts.
- **Switch/Toggle Components**: Use helper functions from `tests/utils/ui-helpers.ts` (`clickSwitch`, `expectSwitchState`, `toggleSwitch`) for reliable interactions. Never use `{ force: true }` or direct clicks on hidden inputs.
- **Clarity**: Use descriptive test and step titles that clearly state the intent. Add comments only to explain complex logic or non-obvious interactions.


### Test Structure
- **Imports**: Start with `import { test, expect } from '@playwright/test';`.
- **Organization**: Group related tests for a feature under a `test.describe()` block.
- **Hooks**: Use `beforeEach` for setup actions common to all tests in a `describe` block (e.g., navigating to a page).
- **Titles**: Follow a clear naming convention, such as `Feature - Specific action or scenario`.


### File Organization
- **Location**: Store all test files in the `tests/` directory.
- **Naming**: Use the convention `<feature-or-page>.spec.ts` (e.g., `login.spec.ts`, `search.spec.ts`).
- **Scope**: Aim for one test file per major application feature or page.

### Assertion Best Practices
- **UI Structure**: Use `toMatchAriaSnapshot` to verify the accessibility tree structure of a component. This provides a comprehensive and accessible snapshot.
- **Element Counts**: Use `toHaveCount` to assert the number of elements found by a locator.
- **Text Content**: Use `toHaveText` for exact text matches and `toContainText` for partial matches.
- **Navigation**: Use `toHaveURL` to verify the page URL after an action.
- **Switch States**: Use `expectSwitchState(locator, boolean)` to verify toggle states. This is more reliable than `toBeChecked()` directly.

### Switch/Toggle Interaction Patterns

Switch components use a hidden `<input>` with styled siblings, requiring special handling:

```typescript
import { clickSwitch, expectSwitchState, toggleSwitch } from './utils/ui-helpers';

// ✅ RECOMMENDED: Click switch with helper
const aclSwitch = page.getByRole('switch', { name: /acl/i });
await clickSwitch(aclSwitch);

// ✅ RECOMMENDED: Assert switch state
await expectSwitchState(aclSwitch, true); // Checked

// ✅ RECOMMENDED: Toggle and verify state change
const newState = await toggleSwitch(aclSwitch);
console.log(`Switch is now ${newState ? 'enabled' : 'disabled'}`);

// ❌ AVOID: Direct click on hidden input
await aclSwitch.click(); // May fail in WebKit/Firefox

// ❌ AVOID: Force clicking (anti-pattern)
await aclSwitch.click({ force: true }); // Bypasses real user behavior

// ❌ AVOID: Hard-coded waits
await page.waitForTimeout(500); // Non-deterministic, slows tests
```

**When to Use**:
- Settings pages with enable/disable toggles
- Security dashboard module switches (CrowdSec, ACL, WAF, Rate Limiting)
- Access lists and configuration toggles
- Any UI component using the `Switch` primitive from shadcn/ui

**References**:
- [Helper Implementation](../../tests/utils/ui-helpers.ts)
- [QA Report](../../docs/reports/qa_report.md)

### Testing Scope: E2E vs Integration

**CRITICAL:** Playwright E2E tests verify **UI/UX functionality** on the Charon management interface (port 8080). They should NOT test middleware enforcement behavior.

#### What E2E Tests SHOULD Cover

✅ **User Interface Interactions:**
- Form submissions and validation
- Navigation and routing
- Visual state changes (toggles, badges, status indicators)
- Authentication flows (login, logout, session management)
- CRUD operations via the management API
- Responsive design (mobile vs desktop layouts)
- Accessibility (ARIA labels, keyboard navigation)

✅ **Example E2E Assertions:**
```typescript
// GOOD: Testing UI state
await expect(aclToggle).toBeChecked();
await expect(statusBadge).toHaveText('Active');
await expect(page).toHaveURL('/proxy-hosts');

// GOOD: Testing API responses in management interface
const response = await request.post('/api/v1/proxy-hosts', { data: hostConfig });
expect(response.ok()).toBeTruthy();
```

#### What E2E Tests should NOT Cover

❌ **Middleware Enforcement Behavior:**
- Rate limiting blocking requests (429 responses)
- ACL denying access based on IP rules (403 responses)
- WAF blocking malicious payloads (SQL injection, XSS)
- CrowdSec IP bans

❌ **Example Wrong E2E Assertions:**
```typescript
// BAD: Testing middleware behavior (rate limiting)
for (let i = 0; i < 6; i++) {
  await request.post('/api/v1/emergency/reset');
}
expect(response.status()).toBe(429); // ❌ This tests Caddy middleware

// BAD: Testing WAF blocking
await request.post('/api/v1/data', { data: "'; DROP TABLE users--" });
expect(response.status()).toBe(403); // ❌ This tests Coraza WAF
```

#### Integration Tests for Middleware

Middleware enforcement is verified by **integration tests** in `backend/integration/`:

- `cerberus_integration_test.go` - Overall security suite behavior
- `coraza_integration_test.go` - WAF blocking (SQL injection, XSS)
- `crowdsec_integration_test.go` - IP reputation and bans
- `rate_limit_integration_test.go` - Request throttling

These tests run in Docker Compose with full Caddy+Cerberus stack and are executed in separate CI workflows.

#### When to Skip Tests

Use `test.skip()` for tests that require middleware enforcement:

```typescript
test('should rate limit after 5 attempts', async ({ request }) => {
  test.skip(
    true,
    'Rate limiting enforced via Cerberus middleware (port 80). Verified in integration tests (backend/integration/).'
  );
  // Test body...
});
```

**Skip Reason Template:**
```
"[Behavior] enforced via Cerberus middleware (port 80). Verified in integration tests (backend/integration/)."
```


## Example Test Structure

```typescript
import { test, expect } from '@playwright/test';

test.describe('Movie Search Feature', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the application before each test
    await page.goto('https://debs-obrien.github.io/playwright-movies-app');
  });

  test('Search for a movie by title', async ({ page }) => {
    await test.step('Activate and perform search', async () => {
      await page.getByRole('search').click();
      const searchInput = page.getByRole('textbox', { name: 'Search Input' });
      await searchInput.fill('Garfield');
      await searchInput.press('Enter');
    });

    await test.step('Verify search results', async () => {
      // Verify the accessibility tree of the search results
      await expect(page.getByRole('main')).toMatchAriaSnapshot(`
        - main:
          - heading "Garfield" [level=1]
          - heading "search results" [level=2]
          - list "movies":
            - listitem "movie":
              - link "poster of The Garfield Movie The Garfield Movie rating":
                - /url: /playwright-movies-app/movie?id=tt5779228&page=1
                - img "poster of The Garfield Movie"
                - heading "The Garfield Movie" [level=2]
      `);
    });
  });
});
```

## Test Execution Strategy

1. **Initial Run**: Execute tests with `npx playwright test --project=chromium`
2. **Debug Failures**: Analyze test failures and identify root causes
3. **Iterate**: Refine locators, assertions, or test logic as needed
4. **Validate**: Ensure tests pass consistently and cover the intended functionality
5. **Report**: Provide feedback on test results and any issues discovered

### Execution Constraints

- **No Truncation**: Never pipe Playwright test output through `head`, `tail`, or other truncating commands. Playwright runs interactively and requires user input to quit when piped, causing the command to hang indefinitely.
- **Full Output**: Always capture the complete test output to analyze failures accurately.

## Quality Checklist

Before finalizing tests, ensure:
- [ ] All locators are accessible and specific and avoid strict mode violations
- [ ] Tests are grouped logically and follow a clear structure
- [ ] Assertions are meaningful and reflect user expectations
- [ ] Tests follow consistent naming conventions
- [ ] Code is properly formatted and commented
