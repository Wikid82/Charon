import { test as playwrightTest, expect as playwrightExpect } from '@playwright/test';

type PlaywrightTest = typeof playwrightTest;
type PlaywrightExpect = typeof playwrightExpect;

let test: PlaywrightTest = playwrightTest;
let expect: PlaywrightExpect = playwrightExpect;

if (process.env.PLAYWRIGHT_COVERAGE === '1') {
	const coverage = await import('@bgotink/playwright-coverage');
	test = coverage.test as unknown as PlaywrightTest;
	expect = coverage.expect as unknown as PlaywrightExpect;
}

export { test, expect };
