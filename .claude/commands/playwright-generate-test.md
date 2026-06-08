# Generate Playwright Test

Generate a Playwright test for the following scenario:

$ARGUMENTS

## Instructions

- If no scenario was provided above, ask the user to describe the user flow to test.
- DO NOT generate test code prematurely without first exploring the app.
- Navigate through the application to understand the actual UI before writing tests.
- Use role-based locators (`getByRole`, `getByLabel`, `getByText`) — never CSS selectors.
- Use `test.step()` to group related interactions.
- Use `toMatchAriaSnapshot` for accessibility verification.
- Follow existing patterns in `tests/` for fixtures and setup.
- Save generated test file in the `tests/` directory.
- Execute the test file and iterate until the test passes.

## Constraints

- **NO HARDCODED WAITS**: Use Playwright's auto-waiting, not `page.waitForTimeout()`.
- **ACCESSIBILITY**: Include `toMatchAriaSnapshot` assertions.
- **NEVER TRUNCATE**: Do not pipe Playwright output through `head` or `tail`.
- Run with: `npx playwright test <test-file> --project=firefox`
