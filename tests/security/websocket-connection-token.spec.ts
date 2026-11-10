/**
 * WebSocket connection list and the session token
 *
 * A live log socket can be opened with the session token in the query string.
 * The connection list is an admin diagnostic, so it must describe each
 * connection without ever echoing that token back.
 *
 * The token is only used inside `includes()` checks and is never logged or
 * attached to an assertion message.
 */

import { test, expect } from '@playwright/test';
import { getAuthTokenFromPage } from '../utils/api-helpers';

const LIVE_LOG_PATH = '/api/v1/logs/live';
const CONNECTIONS_PATH = '/api/v1/websocket/connections';

interface ConnectionListBody {
  connections: unknown[];
  count: number;
}

test.describe('WebSocket connection list', () => {
  test.describe.configure({ mode: 'serial' });

  test('Connection list never shows the token a socket connected with', async ({ page, request }) => {
    let sessionToken = '';

    await test.step('Load the app and read the signed-in session token', async () => {
      await page.goto('/');
      sessionToken = await getAuthTokenFromPage(page);
      expect(sessionToken).not.toBe('');
    });

    await test.step('Open a live log socket that carries the token in its query string', async () => {
      const opened = await page.evaluate(
        ({ path, token }) =>
          new Promise<boolean>((resolve) => {
            const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
            const socket = new WebSocket(
              `${scheme}://${location.host}${path}?token=${encodeURIComponent(token)}`
            );
            (window as Window & { __e2eLogSocket?: WebSocket }).__e2eLogSocket = socket;
            socket.onopen = () => resolve(true);
            socket.onerror = () => resolve(false);
          }),
        { path: LIVE_LOG_PATH, token: sessionToken }
      );
      expect(opened).toBe(true);
    });

    await test.step('The connection is listed while the token is absent from the response body', async () => {
      let rawBody = '';

      await expect
        .poll(async () => {
          const response = await request.get(CONNECTIONS_PATH);
          expect(response.ok()).toBeTruthy();
          rawBody = await response.text();
          const parsed = JSON.parse(rawBody) as ConnectionListBody;
          return parsed.count;
        })
        .toBeGreaterThan(0);

      expect(rawBody.includes(sessionToken)).toBe(false);
    });

    await test.step('Close the socket opened for the check', async () => {
      await page.evaluate(() => {
        const win = window as Window & { __e2eLogSocket?: WebSocket };
        win.__e2eLogSocket?.close();
        delete win.__e2eLogSocket;
      });
    });
  });
});
