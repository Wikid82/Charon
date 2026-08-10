/**
 * "What's New" Changelog Modal E2E Tests
 *
 * Covers the per-user "What's New" modal: it appears after a version bump
 * for users who haven't seen the current version's changelog, and can be
 * snoozed, dismissed, opted out of, and revisited from Settings.
 *
 * STATUS: Commit 5 (hardening) — the backend (`internal/changelog`, User
 * model fields, `/api/v1/changelog/*` routes) and frontend (`WhatsNewModal`,
 * `useChangelog` hooks, Layout mount, Appearance Settings toggle/revisit
 * link) landed in Commits 2-4. These scenarios were originally written as
 * `test(...)` in Commit 1 to lock in names/structure before the
 * implementation existed; they are now real, passing `test(...)` cases
 * verified against that implementation, per `docs/plans/current_spec.md`
 * §4/§6 and the design doc
 * `docs/superpowers/specs/2026-07-28-whats-new-changelog-design.md`.
 *
 * This was the first use of `test.fixme` in this repo — kept simple and
 * copyable for future features that adopt the same pattern.
 *
 * @see /projects/Charon/docs/plans/current_spec.md - §4 Phase 1, §6 Commit 1/5
 * @see /projects/Charon/docs/superpowers/specs/2026-07-28-whats-new-changelog-design.md
 * @see /projects/Charon/frontend/src/components/dialogs/WhatsNewModal.tsx
 *
 * ---------------------------------------------------------------------------
 * FIXTURE INJECTION MECHANISM (applied for every real E2E run of this file)
 * ---------------------------------------------------------------------------
 *
 * `backend/internal/changelog/changelog.go` embeds
 * `backend/internal/changelog/data/changelog.json` via `go:embed` — the data
 * is baked into the binary at COMPILE time. There is no runtime file I/O, so
 * writing a fixture file into an already-running container (or into the repo
 * checkout after the E2E image was built) has zero effect. The only way to
 * get deterministic fixture data into a running E2E backend is:
 *
 *   1. Before building the E2E Docker image, copy this file's contents over
 *      the committed `[]` placeholder at
 *      `backend/internal/changelog/data/changelog.json` (never commit that
 *      overwrite — it must be reverted, e.g. via
 *      `git checkout -- backend/internal/changelog/data/changelog.json`,
 *      immediately after the image build captures it).
 *   2. Rebuild the E2E image so `go:embed` picks up the fixture
 *      (`.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`, or the
 *      equivalent `docker build` step in CI before
 *      `docker-compose.playwright-ci.yml`/`docker-compose.playwright-local.yml`
 *      brings the container up).
 *   3. Set `CHARON_CHANGELOG_VERSION` in the E2E compose file's `environment:`
 *      block (`.docker/compose/docker-compose.playwright-local.yml` and
 *      `docker-compose.playwright-ci.yml`) to `FIXTURE_CURRENT_VERSION` below
 *      (`"1.2.0"`, the fixture's newest entry). This is honored because both
 *      compose files already set `CHARON_ENV=e2e`/`CHARON_ENV=test`
 *      (never `production`), which is the override's gate per the design doc.
 *
 * A single fixed `CHARON_CHANGELOG_VERSION` for the whole E2E container
 * lifetime is sufficient for every scenario below — no mid-suite container
 * restart or live env-var change is needed, because "a further version
 * bump" (scenario 5) is already represented by the gap between a test
 * user's `last_seen_version` and the fixed current version; opting out
 * short-circuits `/changelog/status` before entries are even considered
 * (see `changelog_handler.go`'s planned `Status` handler), so it doesn't
 * matter whether more fixture entries exist beyond it.
 *
 * Bonus wrinkle worth knowing: the E2E Docker image is always built with
 * `VERSION=dev` (the `Dockerfile`'s `ARG VERSION=dev` default —
 * `.github/skills/docker-rebuild-e2e-scripts/run.sh` never passes
 * `--build-arg VERSION=...`). Per the design doc's new-user-seeding rule
 * ("skip seeding if `version.Version == 'dev'`"), every user created via
 * `TestDataManager.createUser()` during E2E naturally gets
 * `LastSeenVersion == ""` for free — no extra per-test seeding step is
 * needed to get a user "behind" the fixture's current version; a plain
 * freshly-created test user already qualifies for scenario 1.
 *
 * If a future scenario genuinely needs to change the effective "current
 * version" mid-run (not needed by anything below), that requires either a
 * second container brought up with a different `CHARON_CHANGELOG_VERSION`,
 * or a new dev/test-only per-request override — flag that as a fresh
 * design question rather than improvising it in test code.
 * ---------------------------------------------------------------------------
 */

import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

import { test, expect, loginUser, logoutUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete, waitForModal } from '../utils/wait-helpers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

/**
 * Deterministic changelog fixture (not real git tag history) so these tests
 * stay independent of this repo's actual release tags, per the design
 * doc's "Local & Pre-Merge Testing" section.
 */
const FIXTURE_PATH = join(__dirname, '../fixtures/changelog-fixture.json');
interface ChangelogFixtureEntry {
  version: string;
  date: string;
  features: string[];
  fixes: string[];
  other: string[];
}
const changelogFixture: ChangelogFixtureEntry[] = JSON.parse(readFileSync(FIXTURE_PATH, 'utf-8'));

/** Newest version in the fixture — must match `CHARON_CHANGELOG_VERSION` set on the E2E backend. */
const FIXTURE_CURRENT_VERSION = changelogFixture[changelogFixture.length - 1].version;

test.describe('What\'s New Changelog', () => {
  test(
    'shows the modal on login when the user is behind the current version',
    async ({ page, regularUser }) => {
      await test.step('Log in as a freshly created (never-seen-changelog) user', async () => {
        await loginUser(page, regularUser);
        await waitForLoadingComplete(page);
      });

      await test.step('"What\'s New" modal appears automatically, newest version first', async () => {
        const modal = await waitForModal(page, "What's New");
        await expect(modal.getByRole('heading', { name: FIXTURE_CURRENT_VERSION })).toBeVisible();
      });

      await test.step('Modal lists features and fixes from the fixture data', async () => {
        const modal = page.getByRole('dialog', { name: "What's New" });
        const newest = changelogFixture[changelogFixture.length - 1];
        for (const feature of newest.features) {
          await expect(modal.getByText(feature)).toBeVisible();
        }
        for (const fix of newest.fixes) {
          await expect(modal.getByText(fix)).toBeVisible();
        }
      });

      await test.step('Accessibility structure of the modal is verified', async () => {
        const modal = page.getByRole('dialog', { name: "What's New" });
        await expect(modal).toMatchAriaSnapshot();
      });
    }
  );

  test(
    '"Remind Me Next Time" snoozes the modal without updating last_seen_version',
    async ({ page, regularUser }) => {
      await test.step('Log in and dismiss via "Remind Me Next Time"', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('button', { name: 'Remind Me Next Time' }).click();
        await expect(modal).not.toBeVisible();
      });

      await test.step('Log out and back in — modal reappears', async () => {
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForModal(page, "What's New");
      });
    }
  );

  test(
    '"Got It, Thanks" permanently dismisses the modal for the current version',
    async ({ page, regularUser }) => {
      await test.step('Log in and dismiss via "Got It, Thanks"', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('button', { name: 'Got It, Thanks' }).click();
        await expect(modal).not.toBeVisible();
      });

      await test.step('Log out and back in — modal does not reappear', async () => {
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForLoadingComplete(page);
        await expect(page.getByRole('dialog', { name: "What's New" })).not.toBeVisible();
      });
    }
  );

  test(
    'X icon / backdrop click has the same effect as "Remind Me Next Time"',
    async ({ page, regularUser }) => {
      await test.step('Log in and close the modal via the X icon', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('button', { name: /close/i }).click();
        await expect(modal).not.toBeVisible();
      });

      await test.step('Log out and back in — modal reappears (snoozed, not dismissed)', async () => {
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForModal(page, "What's New");
      });

      await test.step('Backdrop click behaves identically', async () => {
        // The previous step deliberately left the modal open (to prove it
        // reappeared after the X-icon snooze) — reuse it here instead of
        // logging out/in again first, which would just get blocked by this
        // still-open modal's overlay before ever reaching the Logout button.
        const modal = page.getByRole('dialog', { name: "What's New" });
        await expect(modal).toBeVisible();
        // Click outside the dialog panel to trigger the backdrop dismiss.
        await page.mouse.click(4, 4);
        await expect(modal).not.toBeVisible();

        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForModal(page, "What's New");
      });
    }
  );

  test(
    'checking the opt-out checkbox on any dismiss path suppresses the modal on future logins',
    async ({ page, regularUser }) => {
      await test.step('Log in, check opt-out, and snooze', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('checkbox', { name: /don't show me update notifications/i }).check();
        await modal.getByRole('button', { name: 'Remind Me Next Time' }).click();
        await expect(modal).not.toBeVisible();
      });

      await test.step('Modal stays suppressed on subsequent logins, including after a further version bump', async () => {
        // The fixture already has newer unseen entries beyond this user's
        // last_seen_version (still "") — that gap stands in for "a further
        // version bump" without needing to change CHARON_CHANGELOG_VERSION
        // mid-run; ChangelogOptOut short-circuits /changelog/status
        // regardless of how many unseen entries exist. See the file-level
        // "FIXTURE INJECTION MECHANISM" comment for why this is sufficient.
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForLoadingComplete(page);
        await expect(page.getByRole('dialog', { name: "What's New" })).not.toBeVisible();
      });
    }
  );

  test(
    'Appearance Settings toggle re-enables the modal after opting out',
    async ({ page, regularUser }) => {
      await test.step('Opt out via the modal checkbox', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('checkbox', { name: /don't show me update notifications/i }).check();
        await modal.getByRole('button', { name: 'Remind Me Next Time' }).click();
      });

      await test.step('Navigate to Appearance Settings and flip the toggle back on', async () => {
        // Navigate via in-app links, not page.goto() — goto() performs a
        // full browser navigation (hard reload) for any URL, which would
        // remount WhatsNewModal and defeat the very thing this scenario
        // checks: that opting back in mid-session (no reload) is what
        // causes the modal to resurface, same-session, over this page.
        // Case-sensitive "Settings" (capital S) to avoid matching the
        // unrelated theme-toggle button ("...Open appearance settings.").
        await page.getByRole('button', { name: /Settings/ }).click();
        await page.getByRole('link', { name: /system/i }).click();
        await waitForLoadingComplete(page);
        await page.getByRole('link', { name: /appearance/i }).click();
        await waitForLoadingComplete(page);

        const toggle = page.getByRole('checkbox', { name: /show update notifications/i });
        await expect(toggle).not.toBeChecked();
        // Plain click + an auto-retrying assertion, rather than `.check()`'s
        // own built-in pre/post verification: `checked` here is a controlled
        // prop driven by `!user?.changelog_opt_out`, which only flips after
        // the opt-in mutation's server round trip and a follow-up
        // `refetchUser()` — `.check()`'s tighter internal check can flag
        // "did not change state" against the async-updated element even
        // when the DOM settles correctly moments later (confirmed via
        // screenshot/snapshot on a prior run: checkbox ended up correctly
        // checked, just after `.check()` had already given up).
        await toggle.click();
        await expect(toggle).toBeChecked();
      });

      await test.step('Modal reappears on the next qualifying login', async () => {
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForModal(page, "What's New");
      });
    }
  );

  test(
    '"What\'s New" revisit link in Appearance Settings opens browse mode without affecting dismissal state',
    async ({ page, regularUser }) => {
      await test.step('Log in and permanently dismiss the auto-shown modal first', async () => {
        await loginUser(page, regularUser);
        const modal = await waitForModal(page, "What's New");
        await modal.getByRole('button', { name: 'Got It, Thanks' }).click();
        await expect(modal).not.toBeVisible();
      });

      await test.step('Open the revisit link from Appearance Settings', async () => {
        await page.goto('/settings/appearance');
        await waitForLoadingComplete(page);

        const ackRequestPromise = page.waitForRequest(
          (req) => req.url().includes('/api/v1/changelog/ack'),
          { timeout: 2000 }
        ).catch(() => null);

        await page.getByRole('button', { name: "What's New" }).click();
        const modal = page.getByRole('dialog', { name: "What's New" });
        await expect(modal).toBeVisible();

        // Browse mode fetches the full history via /changelog/all,
        // independent of last_seen_version — every fixture version shows,
        // not just entries newer than what the user already acknowledged.
        for (const entry of changelogFixture) {
          await expect(modal.getByRole('heading', { name: entry.version })).toBeVisible();
        }

        // Closing a voluntary revisit must never call ack. Disambiguate from
        // the dialog's own X icon, which also has an accessible name of
        // "Close" (aria-label, vs. this footer button's visible text) — the
        // footer button renders first in DOM order (see WhatsNewModal.tsx).
        await modal.getByRole('button', { name: 'Close' }).first().click();
        await expect(modal).not.toBeVisible();
        expect(await ackRequestPromise).toBeNull();
      });

      await test.step('Dismissal state from the earlier "Got It, Thanks" click is unchanged', async () => {
        await logoutUser(page);
        await loginUser(page, regularUser);
        await waitForLoadingComplete(page);
        await expect(page.getByRole('dialog', { name: "What's New" })).not.toBeVisible();
      });
    }
  );
});
