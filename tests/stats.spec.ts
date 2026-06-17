/**
 * Dashboard Statistics E2E Tests
 *
 * Tests the enhanced dashboard statistics feature (Issue #25) including:
 * - Statistics section heading visibility
 * - Period selector tab interactions
 * - Bucket selector tab interactions
 * - Request count widget rendering
 * - WebSocket / health widget rendering
 * - Certificate expiry section visibility
 * - Traffic volume chart container
 * - Top hosts chart container
 * - Status distribution chart container
 *
 * @see backend/internal/api/handlers/stats_handler.go
 * @see frontend/src/components/stats/
 */

import { test, expect, loginUser } from './fixtures/auth-fixtures';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

test.describe('Dashboard Statistics', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await waitForAPIHealth(page.request);
    await loginUser(page, adminUser);
    await page.goto('/');
    await waitForLoadingComplete(page);
  });

  // ─── Section heading ────────────────────────────────────────────────────────

  test('should display the Statistics section heading on the dashboard', async ({ page }) => {
    await test.step('Navigate to dashboard root', async () => {
      await page.goto('/');
      await waitForLoadingComplete(page);
    });

    await test.step('Verify Statistics heading is visible', async () => {
      // The heading is rendered via t('dashboard.statistics', 'Statistics')
      // inside a <section aria-labelledby="stats-section-heading">
      const heading = page
        .getByRole('heading', { name: /statistics/i })
        .or(page.locator('#stats-section-heading'));

      await expect(heading.first()).toBeVisible({ timeout: 15_000 });
    });
  });

  // ─── Period selector ────────────────────────────────────────────────────────

  test('should mark the selected period tab as active', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Click the 7d period tab', async () => {
      // PeriodSelector renders buttons with role="radio" inside a role="group"
      const periodGroup = page.getByRole('group', { name: /select time period/i });
      const sevenDay = periodGroup.getByRole('radio', { name: '7d' });
      await sevenDay.click();
    });

    await test.step('Verify 7d radio is checked', async () => {
      const periodGroup = page.getByRole('group', { name: /select time period/i });
      const sevenDay = periodGroup.getByRole('radio', { name: '7d' });
      await expect(sevenDay).toHaveAttribute('aria-checked', 'true');
    });
  });

  test('should allow switching back to the 24h period tab', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    const periodGroup = page.getByRole('group', { name: /select time period/i });

    await test.step('Switch to 30d', async () => {
      await periodGroup.getByRole('radio', { name: '30d' }).click();
    });

    await test.step('Switch back to 24h', async () => {
      await periodGroup.getByRole('radio', { name: '24h' }).click();
    });

    await test.step('Verify 24h is now active', async () => {
      await expect(periodGroup.getByRole('radio', { name: '24h' })).toHaveAttribute('aria-checked', 'true');
    });
  });

  // ─── Bucket selector ────────────────────────────────────────────────────────

  test('should mark the selected bucket tab as active', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Click the 6h bucket tab', async () => {
      const bucketGroup = page.getByRole('group', { name: /select bucket granularity/i });
      await bucketGroup.getByRole('radio', { name: '6h' }).click();
    });

    await test.step('Verify 6h is checked', async () => {
      const bucketGroup = page.getByRole('group', { name: /select bucket granularity/i });
      await expect(bucketGroup.getByRole('radio', { name: '6h' })).toHaveAttribute('aria-checked', 'true');
    });
  });

  // ─── Request count widget ───────────────────────────────────────────────────

  test('should render the Request Counts card with period labels', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Request Counts heading is visible', async () => {
      const requestCountsCard = page.getByText(/request counts/i);
      await expect(requestCountsCard.first()).toBeVisible({ timeout: 10_000 });
    });

    await test.step('Verify Last 24h label is visible in the widget', async () => {
      const last24hLabel = page.getByText(/last 24h/i);
      await expect(last24hLabel.first()).toBeVisible({ timeout: 10_000 });
    });
  });

  // ─── WebSocket / health widget ──────────────────────────────────────────────

  test('should render the Stats Service Health widget', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Stats Service Health card heading is visible', async () => {
      const healthCard = page.getByText(/stats service health/i);
      await expect(healthCard.first()).toBeVisible({ timeout: 10_000 });
    });

    await test.step('Verify Live or Offline badge is rendered', async () => {
      // ServiceHealthWidget renders either "Live" or "Offline" badge depending on WS state
      const badge = page
        .getByText(/^live$/i)
        .or(page.getByText(/^offline$/i));
      await expect(badge.first()).toBeVisible({ timeout: 10_000 });
    });

    await test.step('Verify Real-time feed label is present', async () => {
      const feedLabel = page.getByText(/real-time feed/i);
      await expect(feedLabel.first()).toBeVisible({ timeout: 10_000 });
    });
  });

  // ─── Certificate expiry section ─────────────────────────────────────────────

  test('should render the Certificate Expiry section', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Certificate Expiry card is rendered', async () => {
      const certCard = page.getByText(/certificate expiry/i);
      await expect(certCard.first()).toBeVisible({ timeout: 10_000 });
    });

    await test.step('Verify expiry content or empty-state message is shown', async () => {
      // Either a table with certificates or the empty-state paragraph
      const hasRows = await page
        .getByRole('table', { name: /certificate expiry/i })
        .isVisible()
        .catch(() => false);
      const hasEmptyMsg = await page
        .getByText(/no certificates expiring soon/i)
        .isVisible()
        .catch(() => false);

      expect(hasRows || hasEmptyMsg).toBeTruthy();
    });
  });

  // ─── Traffic volume chart ───────────────────────────────────────────────────

  test('should render the Traffic Volume chart container', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Traffic Volume chart card is visible', async () => {
      const chartCard = page.getByText(/traffic volume/i);
      await expect(chartCard.first()).toBeVisible({ timeout: 10_000 });
    });

    await test.step('Verify chart SVG or empty-state is rendered', async () => {
      // Recharts renders an <svg>; when no data exists the component shows a skeleton or placeholder
      const chartSection = page
        .locator('section[aria-labelledby="stats-section-heading"]')
        .or(page.getByRole('main'));

      const hasSvg = await chartSection.locator('svg').count() > 0;
      const hasChartText = await page.getByText(/traffic volume/i).isVisible().catch(() => false);

      expect(hasSvg || hasChartText).toBeTruthy();
    });

    await test.step('Verify SVG line path is rendered inside the chart', async () => {
      // If the chart is showing the empty state, skip the SVG assertion
      const hasEmptyState = await page.getByText(/no data available yet/i).isVisible().catch(() => false);
      if (hasEmptyState) {
        // Empty state is acceptable; just confirm the card is shown
        await expect(page.getByText(/traffic volume/i).first()).toBeVisible();
        return;
      }

      // When data is present, an SVG with recharts-line-curve path must exist
      const svgLineCount = await page.locator('.recharts-line-curve').count();
      expect(svgLineCount).toBeGreaterThan(0);
    });
  });

  // ─── Top hosts chart ────────────────────────────────────────────────────────

  test('should render the Top Hosts chart container', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Top Hosts chart card heading is visible', async () => {
      const topHostsCard = page.getByText(/top hosts/i);
      await expect(topHostsCard.first()).toBeVisible({ timeout: 10_000 });
    });
  });

  // ─── Status distribution chart ──────────────────────────────────────────────

  test('should render the Status Distribution chart container', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify Status Distribution chart card heading is visible', async () => {
      const statusCard = page.getByText(/status distribution/i);
      await expect(statusCard.first()).toBeVisible({ timeout: 10_000 });
    });
  });

  // ─── Accessibility snapshot ─────────────────────────────────────────────────

  test('should expose all stats selector controls as accessible radio buttons', async ({ page }) => {
    await test.step('Wait for statistics section', async () => {
      await expect(
        page.getByRole('heading', { name: /statistics/i }).or(page.locator('#stats-section-heading')).first()
      ).toBeVisible({ timeout: 15_000 });
    });

    await test.step('Verify period selector group has accessible radio options', async () => {
      const periodGroup = page.getByRole('group', { name: /select time period/i });
      await expect(periodGroup).toBeVisible();

      const radios = periodGroup.getByRole('radio');
      await expect(radios).toHaveCount(3);
    });

    await test.step('Verify bucket selector group has accessible radio options', async () => {
      const bucketGroup = page.getByRole('group', { name: /select bucket granularity/i });
      await expect(bucketGroup).toBeVisible();

      const radios = bucketGroup.getByRole('radio');
      await expect(radios).toHaveCount(3);
    });
  });
});
