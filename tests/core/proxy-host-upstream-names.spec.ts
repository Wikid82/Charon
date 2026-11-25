/**
 * Proxy hosts that forward to a container name.
 *
 * The upstream-echo service (see the compose files) answers one fixed body on port 8080 over the compose network. These
 * tests save a proxy host that forwards to that name and send requests through the Caddy hop from inside the Charon
 * container, so the name is resolved the way it is in production:
 *
 * - the name is served and the request reaches the container (200 with the fixed body),
 * - the running configuration sends every reverse_proxy through the upstream name resolver on loopback, with HTTP
 *   as the transport protocol.
 */

import { test, expect, loginUser } from "../fixtures/auth-fixtures";
import {
  fetchFromContainer,
  findE2EContainer,
  execInContainer,
} from "../utils/container-exec";
import { waitForLoadingComplete } from "../utils/wait-helpers";

const UPSTREAM_NAME = "upstream-echo";
const UPSTREAM_PORT = 8080;
const UPSTREAM_BODY = "upstream-echo ok";
const RESOLVER_ADDRESS = /^udp\/127\.0\.0\.1:\d+$/;

interface ReverseProxyHandler {
  handler: "reverse_proxy";
  transport?: {
    protocol?: string;
    resolver?: { addresses?: string[] };
  };
}

/** Collects every reverse_proxy handler object in a Caddy JSON config, wherever it is nested. */
function collectReverseProxyHandlers(
  node: unknown,
  found: ReverseProxyHandler[] = [],
): ReverseProxyHandler[] {
  if (Array.isArray(node)) {
    for (const item of node) {
      collectReverseProxyHandlers(item, found);
    }
  } else if (node !== null && typeof node === "object") {
    const record = node as Record<string, unknown>;
    if (record.handler === "reverse_proxy") {
      found.push(record as unknown as ReverseProxyHandler);
    }
    for (const value of Object.values(record)) {
      collectReverseProxyHandlers(value, found);
    }
  }
  return found;
}

test.describe("Proxy hosts that forward to a container name", () => {
  let container: string;
  let domain: string;

  test.beforeEach(async ({ page, adminUser, testData }) => {
    container = findE2EContainer();
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);

    const host = await testData.createProxyHost({
      domain: `upstream-name-${Date.now()}.example.test`,
      forwardHost: UPSTREAM_NAME,
      forwardPort: UPSTREAM_PORT,
      name: "Container name upstream host",
    });
    domain = host.domain;
  });

  test("serves a request forwarded to a container name", async () => {
    await test.step("Proxy route answers with the upstream body", async () => {
      await expect
        .poll(
          async () => {
            const response = await fetchFromContainer(container, {
              host: domain,
              path: "/",
            });
            return response.status === 200 && response.body === UPSTREAM_BODY;
          },
          {
            timeout: 30_000,
            message:
              "container-name proxy route should serve the upstream body",
          },
        )
        .toBe(true);
    });

    await test.step("Response is the one returned by the named container", async () => {
      const response = await fetchFromContainer(container, {
        host: domain,
        path: "/",
      });
      expect(response.status).toBe(200);
      expect(response.body).toBe(UPSTREAM_BODY);
    });
  });

  test("loads every reverse_proxy with the loopback upstream name resolver", async () => {
    await test.step("Wait until the route for the new host is loaded", async () => {
      await expect
        .poll(
          async () =>
            (await fetchFromContainer(container, { host: domain, path: "/" }))
              .status,
          {
            timeout: 30_000,
            message:
              "container-name proxy route should be loaded before the config is read",
          },
        )
        .toBe(200);
    });

    await test.step("Every reverse_proxy uses HTTP transport with a loopback resolver", async () => {
      const config: unknown = JSON.parse(
        execInContainer(container, [
          "/app/charon",
          "caddy-admin",
          "get",
          "/config/",
        ]),
      );
      const handlers = collectReverseProxyHandlers(config);

      expect(
        handlers.length,
        "running config should contain reverse_proxy handlers",
      ).toBeGreaterThan(0);
      for (const handler of handlers) {
        expect(
          handler.transport?.protocol,
          "reverse_proxy transport protocol",
        ).toBe("http");
        const addresses = handler.transport?.resolver?.addresses ?? [];
        expect(addresses, "reverse_proxy resolver addresses").toHaveLength(1);
        expect(addresses[0], "reverse_proxy resolver address").toMatch(
          RESOLVER_ADDRESS,
        );
      }
    });
  });
});
