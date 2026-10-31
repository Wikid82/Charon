/**
 * CrowdSec hub offline presets E2E Tests
 *
 * When the CrowdSec hub cannot be reached, the presets list is answered from the
 * built-in and downloaded sources and says so:
 * - `hub_offline: true` shows a passive, muted note (no banner, no button)
 * - hub-backed rows that are not cached and report `available: false` are shown disabled
 * - auto-select skips those rows and picks the first selectable one, or nothing
 * - `hub_offline: false` shows no note
 *
 * The E2E container has no cscli or hub, so the preset list is route-mocked.
 * Every case is `test.fixme` until the UI lands.
 */

import type { Page } from "@playwright/test";
import { test, expect, loginUser } from "../fixtures/auth-fixtures";
import { waitForLoadingComplete } from "../utils/wait-helpers";
import { PRESETS_ROUTE, fulfillJson } from "../utils/crowdsec-stubs";

const PRESETS_PATH = "/api/v1/admin/crowdsec/presets";
const HUB_OFFLINE_NOTE =
  "Showing built-in and downloaded presets. The hub list was not loaded.";

interface PresetRowFixture {
  slug: string;
  title: string;
  source: "hub" | "charon-curated";
  requires_hub: boolean;
  available: boolean;
  cached: boolean;
}

/** Hub-backed row that is not cached and cannot be fetched while the hub is offline. */
const OFFLINE_HUB_ROW: PresetRowFixture = {
  slug: "crowdsecurity/e2e-offline-hub-preset",
  title: "E2E Offline Hub Preset",
  source: "hub",
  requires_hub: true,
  available: false,
  cached: false,
};

/** Built-in preset that needs no hub, so it stays selectable offline. */
const SELECTABLE_ROW: PresetRowFixture = {
  slug: "crowdsecurity/e2e-selectable-preset",
  title: "E2E Selectable Preset",
  source: "charon-curated",
  requires_hub: false,
  available: true,
  cached: false,
};

/** Second hub-backed row that is not cached and cannot be fetched while the hub is offline. */
const SECOND_OFFLINE_HUB_ROW: PresetRowFixture = {
  slug: "crowdsecurity/e2e-second-offline-hub-preset",
  title: "E2E Second Offline Hub Preset",
  source: "hub",
  requires_hub: true,
  available: false,
  cached: false,
};

/**
 * Stubs GET presets with the given rows and the `hub_offline` flag, and records the
 * slugs of every pull request so tests can assert that no unavailable row was pulled.
 */
async function stubPresetList(
  page: Page,
  presets: PresetRowFixture[],
  hubOffline: boolean,
): Promise<{ pullSlugs: string[] }> {
  const pullSlugs: string[] = [];
  await page.route(PRESETS_ROUTE, async (route) => {
    if (route.request().method() !== "GET") {
      await route.fallback();
      return;
    }
    await fulfillJson(route, { presets, hub_offline: hubOffline });
  });
  await page.route("**/api/v1/admin/crowdsec/presets/pull", async (route) => {
    if (route.request().method() === "POST") {
      const body = route.request().postDataJSON() as { slug?: string };
      pullSlugs.push(body.slug ?? "");
    }
    await route.fallback();
  });
  return { pullSlugs };
}

/**
 * Loads the CrowdSec page and waits for the presets response the test stubbed, so
 * assertions run against the list the page actually received.
 */
async function loadPresetsPage(page: Page): Promise<void> {
  const presetsResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === PRESETS_PATH &&
      response.request().method() === "GET",
  );
  await page.goto("/security/crowdsec");
  await presetsResponse;
  await waitForLoadingComplete(page);
}

/** The preset row (role button) whose accessible name contains the title. */
function presetRow(page: Page, title: string) {
  return page.getByRole("button", { name: new RegExp(title, "i") });
}

test.describe("CrowdSec hub offline presets @security", () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.fixme("should show the muted hub offline note with no banner or button when hub_offline is true", async ({
    page,
  }) => {
    await stubPresetList(page, [SELECTABLE_ROW, OFFLINE_HUB_ROW], true);

    await test.step("Load the presets list served without the hub", async () => {
      await loadPresetsPage(page);
    });

    await test.step("Verify the offline note is shown with its text", async () => {
      const note = page.getByTestId("preset-hub-offline");
      await expect(note).toBeVisible();
      await expect(note).toHaveText(HUB_OFFLINE_NOTE);
    });

    await test.step("Verify the note is passive: no button inside it and no hub unavailable banner", async () => {
      await expect(
        page.getByTestId("preset-hub-offline").getByRole("button"),
      ).toHaveCount(0);
      await expect(page.getByTestId("preset-hub-unavailable")).toHaveCount(0);
      await expect(
        page.getByRole("button", { name: "Retry", exact: true }),
      ).toHaveCount(0);
    });
  });

  test.fixme("should show unavailable hub rows disabled and auto-select the first selectable row", async ({
    page,
  }) => {
    const { pullSlugs } = await stubPresetList(
      page,
      [OFFLINE_HUB_ROW, SELECTABLE_ROW],
      true,
    );

    await test.step("Load the presets list with one unavailable hub row listed first", async () => {
      await loadPresetsPage(page);
    });

    await test.step("Verify the unavailable row is shown disabled and not selected", async () => {
      const offlineRow = presetRow(page, OFFLINE_HUB_ROW.title);
      await expect(offlineRow).toBeVisible();
      await expect(offlineRow).toHaveAttribute("aria-disabled", "true");
      await expect(offlineRow).toHaveAttribute("aria-pressed", "false");
    });

    await test.step("Verify auto-select skipped the unavailable row and picked the first selectable row", async () => {
      await expect(presetRow(page, SELECTABLE_ROW.title)).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    });

    await test.step("Verify clicking the disabled row does not select it", async () => {
      // Force the click past Playwright's aria-disabled actionability check so the handler is exercised.
      await presetRow(page, OFFLINE_HUB_ROW.title).click({ force: true });
      await expect(presetRow(page, OFFLINE_HUB_ROW.title)).toHaveAttribute(
        "aria-pressed",
        "false",
      );
      await expect(presetRow(page, SELECTABLE_ROW.title)).toHaveAttribute(
        "aria-pressed",
        "true",
      );
      expect(pullSlugs).not.toContain(OFFLINE_HUB_ROW.slug);
    });
  });

  test.fixme("should select nothing and show no error when every row is unavailable offline", async ({
    page,
  }) => {
    const { pullSlugs } = await stubPresetList(
      page,
      [OFFLINE_HUB_ROW, SECOND_OFFLINE_HUB_ROW],
      true,
    );

    await test.step("Load the presets list where no row can be used", async () => {
      await loadPresetsPage(page);
    });

    await test.step("Verify both rows are disabled and nothing is selected", async () => {
      for (const row of [OFFLINE_HUB_ROW, SECOND_OFFLINE_HUB_ROW]) {
        await expect(presetRow(page, row.title)).toHaveAttribute(
          "aria-disabled",
          "true",
        );
        await expect(presetRow(page, row.title)).toHaveAttribute(
          "aria-pressed",
          "false",
        );
      }
    });

    await test.step("Verify no error is shown and no pull was sent for either row", async () => {
      await expect(page.getByRole("alert")).toHaveCount(0);
      await expect(page.getByTestId("preset-hub-unavailable")).toHaveCount(0);
      expect(pullSlugs).toEqual([]);
    });
  });

  test.fixme("should show no hub offline note when hub_offline is false", async ({
    page,
  }) => {
    await stubPresetList(page, [SELECTABLE_ROW, OFFLINE_HUB_ROW], false);

    await test.step("Load the presets list served with the hub", async () => {
      await loadPresetsPage(page);
    });

    await test.step("Verify the note is absent and the presets are listed", async () => {
      await expect(presetRow(page, SELECTABLE_ROW.title)).toBeVisible();
      await expect(page.getByTestId("preset-hub-offline")).toHaveCount(0);
    });
  });
});
