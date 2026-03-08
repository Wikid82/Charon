# Playwright: Generate Test

Generate a Playwright test based on a provided scenario.

**Scenario**: $ARGUMENTS (if not provided, ask the user for a scenario)

## Instructions

- DO NOT generate test code prematurely or based solely on the scenario without completing all steps below
- Run each step using Playwright tools before writing the test
- Only after all steps are completed, emit a Playwright TypeScript test using `@playwright/test`
- Save the generated test file in the `tests/` directory
- Execute the test file and iterate until the test passes

## Steps

1. **Navigate** to the relevant page/feature described in the scenario
2. **Explore** the UI elements involved — identify accessible locators (`getByRole`, `getByLabel`, `getByText`)
3. **Perform** the user actions described in the scenario step by step
4. **Observe** the expected outcomes and note assertions needed
5. **Generate** the Playwright TypeScript test based on message history

## Test Quality Standards

- Use `@playwright/test` with `test` and `expect`
- Use role-based locators — never CSS selectors or XPath
- Group interactions with `test.step()` for clarity
- Include `toMatchAriaSnapshot` for structural verification
- No hardcoded waits (`page.waitForTimeout`) — use Playwright's auto-waiting
- Test names must be descriptive: `test('user can create a proxy host with SSL', async ({ page }) => {`

## File Naming

- New tests: `tests/{feature-name}.spec.ts`
- Follow existing naming patterns in `tests/`

## After Generation

Run the test:
```bash
cd /projects/Charon && npx playwright test tests/{your-test}.spec.ts --project=firefox
```

Iterate until the test passes with no flakiness.
