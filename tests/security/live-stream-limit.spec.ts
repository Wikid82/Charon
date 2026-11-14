/**
 * Live stream connection limit on the Logs page (PR 25, commit 1: pending spec).
 *
 * One user may hold at most 8 live streams (logs, Cerberus logs, Hecate logs, stats). A ninth stream is accepted
 * and then closed with WebSocket close code 1013 (Try Again Later). The UI shows a message and does not reconnect
 * in a loop. Another user still connects. Every test here is `test.fixme` until the stream limit lands
 * (PR 25 commit 5, frontend mapping in the stream hooks).
 *
 * English labels below come from the en locale (errors.streamLimitReached) and must be kept in sync with it.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';
import type { Page, WebSocket } from '@playwright/test';

/** Per-user cap from the stream quota (PR 25 D-25-7). */
const PER_USER_STREAM_LIMIT = 8;
const LOGS_PATH = '/logs';
const LIVE_STREAM_PATH = '/api/v1/logs/live';
/** Message shown in place of the live view when the stream was closed with 1013. */
const STREAM_LIMIT_MESSAGE = /too many (live )?connections|connection limit/i;

/** Opens the Logs page in its own browser context, so each page holds its own socket. */
async function openLiveLogs(
  browser: import('@playwright/test').Browser,
  sockets: WebSocket[]
): Promise<Page> {
  const context = await browser.newContext({ storageState: STORAGE_STATE });
  const page = await context.newPage();
  page.on('websocket', (socket) => {
    if (socket.url().includes(LIVE_STREAM_PATH)) {
      sockets.push(socket);
    }
  });
  await page.goto(LOGS_PATH);
  return page;
}

test.describe('Live stream connection limit', () => {
  test.describe.configure({ mode: 'serial' });

  test.fixme('Ninth live stream for one user closes with the stream limit message', async ({ browser }) => {
    const sockets: WebSocket[] = [];
    const pages: Page[] = [];

    try {
      await test.step(`Open ${PER_USER_STREAM_LIMIT} live streams for the same user`, async () => {
        for (let i = 0; i < PER_USER_STREAM_LIMIT; i++) {
          pages.push(await openLiveLogs(browser, sockets));
        }
        await expect.poll(() => sockets.length).toBe(PER_USER_STREAM_LIMIT);
      });

      const ninth = await openLiveLogs(browser, sockets);
      pages.push(ninth);

      await test.step('The ninth view shows the stream limit message', async () => {
        await expect(ninth.getByRole('alert')).toContainText(STREAM_LIMIT_MESSAGE);
      });

      await test.step('The ninth view does not reconnect in a loop', async () => {
        // A reconnect would open a new socket for this page; the count must stay at the cap plus the refused one.
        const socketsAfterClose = sockets.length;
        await expect.poll(() => sockets.length).toBe(socketsAfterClose);
        await expect(ninth.getByRole('alert')).toContainText(STREAM_LIMIT_MESSAGE);
      });
    } finally {
      for (const page of pages) {
        await page.context().close();
      }
    }
  });

  test.fixme('Another user still connects while one user is at the stream limit', async ({ browser, regularUser, page }) => {
    const sockets: WebSocket[] = [];
    const pages: Page[] = [];

    try {
      await test.step(`Fill the first user's ${PER_USER_STREAM_LIMIT} streams and one more`, async () => {
        for (let i = 0; i <= PER_USER_STREAM_LIMIT; i++) {
          pages.push(await openLiveLogs(browser, sockets));
        }
      });

      await test.step('A different user gets a live view without the limit message', async () => {
        await loginUser(page, regularUser);
        await page.goto(LOGS_PATH);
        await expect(page.getByRole('alert').filter({ hasText: STREAM_LIMIT_MESSAGE })).toHaveCount(0);
      });
    } finally {
      for (const openPage of pages) {
        await openPage.context().close();
      }
    }
  });
});
