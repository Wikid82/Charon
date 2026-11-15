/**
 * Live stream connection limit (PR 25, commit 13: end-to-end coverage).
 *
 * One user may hold at most 8 live streams (logs, Cerberus logs, Hecate logs, stats). A ninth stream is accepted
 * and then closed with WebSocket close code 1013 (Try Again Later, reason stream_limit_reached). The UI shows a message
 * and does not reconnect in a loop. Another user still connects.
 *
 * The application-log stream (/api/v1/logs/live) is opened by the live viewer on the Security page once it is switched
 * to "Application logs". That viewer only renders while Cerberus is enabled, so the suite enables it for the run and
 * restores the captured state afterwards.
 *
 * Each page is judged by the frames its application socket receives, not by the viewer's connection badge. A frame
 * shows the server accepted the stream and is sending it.
 *
 * English labels below come from the en locale (errors.streamLimitReached) and must be kept in sync with it.
 */

import { request as playwrightRequest, type APIRequestContext, type Browser, type Page, type WebSocket } from '@playwright/test';
import { test, expect, loginUser, TEST_PASSWORD } from '../fixtures/auth-fixtures';
import { STORAGE_STATE } from '../constants';
import { suppressChangelogModal } from '../utils/api-helpers';
import {
  captureSecurityState,
  restoreSecurityState,
  setSecurityModuleEnabled,
  type SecurityStateSnapshot,
} from '../utils/security-helpers';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
/** Per-user cap from the stream quota (PR 25 D-25-7). */
const PER_USER_STREAM_LIMIT = 8;
const SECURITY_PATH = '/security';
const LIVE_STREAM_PATH = '/api/v1/logs/live';
/** Switches the live viewer on the Security page to application logs (the button's title). */
const APPLICATION_LOGS_TITLE = 'Application logs';
/** Message shown in place of the live view when the stream was closed with 1013. */
const STREAM_LIMIT_MESSAGE = /Too many live connections/;

/** Application-log sockets opened by one user's pages, in opening order, with the frames each has received. */
interface LiveStreams {
  sockets: WebSocket[];
  frames: Map<WebSocket, number>;
}

function newLiveStreams(): LiveStreams {
  return { sockets: [], frames: new Map() };
}

/** Records every application-log socket the page opens, and the frames each one receives. */
function trackApplicationStreams(page: Page, streams: LiveStreams): void {
  page.on('websocket', (socket) => {
    if (!socket.url().includes(LIVE_STREAM_PATH)) {
      return;
    }
    streams.sockets.push(socket);
    streams.frames.set(socket, 0);
    socket.on('framereceived', () => {
      streams.frames.set(socket, (streams.frames.get(socket) ?? 0) + 1);
    });
  });
}

/**
 * Waits until the stream at `position` (1-based, in opening order) has been accepted and is delivering frames.
 *
 * The stream only sends a frame when the server writes a log line, so each poll makes one API request (logged as a
 * handled request). Without it the check would depend on unrelated background traffic reaching the log.
 */
async function expectStreamLive(api: APIRequestContext, streams: LiveStreams, position: number): Promise<void> {
  await expect.poll(() => streams.sockets.length, { message: `stream ${position} should be opened` }).toBeGreaterThanOrEqual(
    position
  );
  const socket = streams.sockets[position - 1];
  await expect
    .poll(
      async () => {
        await api.get('/api/v1/health');
        return streams.frames.get(socket) ?? 0;
      },
      { message: `stream ${position} should deliver frames` }
    )
    .toBeGreaterThan(0);
}

/**
 * Opens the Security page in its own browser context and switches the live viewer to application logs. Pages share the
 * admin's session, so they all count against one user's cap.
 */
async function openApplicationLiveLogs(browser: Browser, streams: LiveStreams): Promise<Page> {
  const context = await browser.newContext({ storageState: STORAGE_STATE });
  const page = await context.newPage();
  trackApplicationStreams(page, streams);
  await page.goto(SECURITY_PATH);
  await page.getByTitle(APPLICATION_LOGS_TITLE).click();
  return page;
}

test.describe('Live stream connection limit', () => {
  test.describe.configure({ mode: 'serial' });

  let api: APIRequestContext;
  let snapshot: SecurityStateSnapshot | undefined;

  test.beforeAll(async () => {
    api = await playwrightRequest.newContext({ baseURL: APP_BASE_URL, storageState: STORAGE_STATE });
    snapshot = await captureSecurityState(api);
    await setSecurityModuleEnabled(api, 'cerberus', true);
  });

  test.afterAll(async () => {
    try {
      if (snapshot) {
        await restoreSecurityState(api, snapshot);
      }
    } finally {
      await api.dispose();
    }
  });

  test('Ninth live stream for one user closes with the stream limit message', async ({ browser }) => {
    const streams = newLiveStreams();
    const pages: Page[] = [];

    try {
      await test.step(`Open ${PER_USER_STREAM_LIMIT} live streams for the same user`, async () => {
        for (let position = 1; position <= PER_USER_STREAM_LIMIT; position++) {
          pages.push(await openApplicationLiveLogs(browser, streams));
          await expectStreamLive(api, streams, position);
        }
      });

      const ninth = await openApplicationLiveLogs(browser, streams);
      pages.push(ninth);

      await test.step('The ninth view shows the stream limit message', async () => {
        await expect(ninth.getByRole('alert').filter({ hasText: STREAM_LIMIT_MESSAGE })).toBeVisible();
      });

      await test.step('The ninth view does not reconnect: the socket count stays at the cap plus the refused one', async () => {
        // A reconnect would open another socket for the ninth page, so the count would rise above the cap plus one.
        await expect.poll(() => streams.sockets.length).toBe(PER_USER_STREAM_LIMIT + 1);
        await expect(ninth.getByRole('alert').filter({ hasText: STREAM_LIMIT_MESSAGE })).toHaveCount(1);
      });
    } finally {
      for (const page of pages) {
        await page.context().close();
      }
    }
  });

  test('Another user still connects while one user is at the stream limit', async ({ browser, regularUser, page }) => {
    const streams = newLiveStreams();
    const otherUserStreams = newLiveStreams();
    const pages: Page[] = [];

    try {
      await test.step(`Fill the first user's ${PER_USER_STREAM_LIMIT} streams and one more`, async () => {
        for (let position = 1; position <= PER_USER_STREAM_LIMIT; position++) {
          pages.push(await openApplicationLiveLogs(browser, streams));
          await expectStreamLive(api, streams, position);
        }
        pages.push(await openApplicationLiveLogs(browser, streams));
        await expect.poll(() => streams.sockets.length).toBe(PER_USER_STREAM_LIMIT + 1);
      });

      await test.step('A different user gets a live view without the limit message', async () => {
        // The regular user is new and would otherwise see the blocking "What's New" dialog over the page.
        await page.goto('/');
        await suppressChangelogModal(page, regularUser.email, TEST_PASSWORD);
        await loginUser(page, regularUser);
        trackApplicationStreams(page, otherUserStreams);
        await page.goto(SECURITY_PATH);
        await page.getByTitle(APPLICATION_LOGS_TITLE).click();
        await expectStreamLive(api, otherUserStreams, 1);
        await expect(page.getByRole('alert').filter({ hasText: STREAM_LIMIT_MESSAGE })).toHaveCount(0);
      });
    } finally {
      for (const openPage of pages) {
        await openPage.context().close();
      }
    }
  });
});
