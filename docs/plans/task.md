Run npx playwright test --project=chromium

Running 55 tests using 1 worker
Running initial setup to create test admin user...
Initial setup completed successfully
Logging in as test user...
Login successful
Auth state saved to /home/runner/work/Charon/Charon/playwright/.auth/user.json
·API Response: 404 {"error":"not found"}
×API Response: 404 {"error":"not found"}
×API Response: 404 {"error":"not found"}
FType select found: true
Number of options: 1
  Option 0: Loading...
Webhook option not found
°··Add button count: 2
Page URL: http://localhost:8080/dns/providers
···°°°××F·××F····××F××F××F××F××F××F··××F···××F························

  1) [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider › Save provider

    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7

    Error Context: test-results/dns-provider-crud-DNS-Prov-3ef77-reate-a-Manual-DNS-provider-chromium/error-context.md

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7

    Error Context: test-results/dns-provider-crud-DNS-Prov-3ef77-reate-a-Manual-DNS-provider-chromium-retry1/error-context.md

    attachment #2: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-crud-DNS-Prov-3ef77-reate-a-Manual-DNS-provider-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-crud-DNS-Prov-3ef77-reate-a-Manual-DNS-provider-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7

    Error Context: test-results/dns-provider-crud-DNS-Prov-3ef77-reate-a-Manual-DNS-provider-chromium-retry2/error-context.md

  2) [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API

    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-crud-DNS-Prov-2082f-ould-list-providers-via-API-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-crud-DNS-Prov-2082f-ould-list-providers-via-API-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29

  3) [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API

    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-crud-DNS-Prov-49999-valid-provider-type-via-API-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-crud-DNS-Prov-49999-valid-provider-type-via-API-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33

  4) [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom

    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-aa4d6-cluding-built-in-and-custom-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-aa4d6-cluding-built-in-and-custom-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29

  5) [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields

    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-cf7e1-should-have-required-fields-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-cf7e1-should-have-required-fields-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30

  6) [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration

    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-0fe47--have-correct-configuration-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-0fe47--have-correct-configuration-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36

  7) [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field

    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-110d1--type-should-have-url-field-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-110d1--type-should-have-url-field-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37

  8) [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields

    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-357a9--have-server-and-key-fields-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-357a9--have-server-and-key-fields-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37

  9) [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field

    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36

    attachment #1: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-2c379-uld-have-command-path-field-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-2c379-uld-have-command-path-field-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36

  10) [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search › Verify options can be navigated with keyboard

    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7

    Error Context: test-results/dns-provider-types-DNS-Pro-b3c1a-vider-types-based-on-search-chromium/error-context.md

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7

    Error Context: test-results/dns-provider-types-DNS-Pro-b3c1a-vider-types-based-on-search-chromium-retry1/error-context.md

    attachment #2: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-b3c1a-vider-types-based-on-search-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-b3c1a-vider-types-based-on-search-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7

    Error Context: test-results/dns-provider-types-DNS-Pro-b3c1a-vider-types-based-on-search-chromium-retry2/error-context.md

  11) [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected › Verify Script path/command field appears

    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18

    Error Context: test-results/dns-provider-types-DNS-Pro-b805a-hen-Script-type-is-selected-chromium/error-context.md

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18

    Error Context: test-results/dns-provider-types-DNS-Pro-b805a-hen-Script-type-is-selected-chromium-retry1/error-context.md

    attachment #2: trace (application/zip) ─────────────────────────────────────────────────────────
    test-results/dns-provider-types-DNS-Pro-b805a-hen-Script-type-is-selected-chromium-retry1/trace.zip
    Usage:

        npx playwright show-trace test-results/dns-provider-types-DNS-Pro-b805a-hen-Script-type-is-selected-chromium-retry1/trace.zip

    ────────────────────────────────────────────────────────────────────────────────────────────────

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────

    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18

    Error Context: test-results/dns-provider-types-DNS-Pro-b805a-hen-Script-type-is-selected-chromium-retry2/error-context.md

  11 failed
    [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider
    [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API
    [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API
    [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom
    [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields
    [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration
    [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field
    [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields
    [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field
    [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search
    [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected
  4 skipped
  40 passed (2.0m)
Error:   1) [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider › Save provider
    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7
Error:   1) [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider › Save provider

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7
Error:   1) [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider › Save provider

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(locator).not.toBeVisible() failed

    Locator:  getByRole('dialog')
    Expected: not visible
    Received: visible
    Timeout:  10000ms

    Call log:
      - Expect "not toBeVisible" with timeout 10000ms
      - waiting for getByRole('dialog')
        14 × locator resolved to <div role="dialog" tabindex="-1" id="radix-_r_0_" data-state="open" aria-labelledby="radix-_r_1_" class="fixed left-[50%] top-[50%] z-50 w-full translate-x-[-50%] translate-y-[-50%] bg-surface-elevated border border-border rounded-xl shadow-xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:sli…>…</div>
           - unexpected value "visible"


      65 |
      66 |         // Wait for dialog to close (indicates success)
    > 67 |         await expect(dialog).not.toBeVisible({ timeout: 10000 });
         |                                  ^
      68 |       });
      69 |
      70 |       await test.step('Verify success', async () => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:67:34
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:40:7
Error:   2) [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API
    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29
Error:   2) [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29
Error:   2) [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeTruthy()

    Received: false

      449 |     test('should list providers via API', async ({ request }) => {
      450 |       const response = await request.get('/api/v1/dns-providers');
    > 451 |       expect(response.ok()).toBeTruthy();
          |                             ^
      452 |
      453 |       const data = await response.json();
      454 |       // Response should be an array (possibly empty)
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:451:29
Error:   3) [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API
    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33
Error:   3) [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33
Error:   3) [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBe(expected) // Object.is equality

    Expected: 400
    Received: 404

      489 |
      490 |       // Should return 400 Bad Request for invalid type
    > 491 |       expect(response.status()).toBe(400);
          |                                 ^
      492 |     });
      493 |
      494 |     test('should get single provider via API', async ({ request }) => {
        at /home/runner/work/Charon/Charon/tests/dns-provider-crud.spec.ts:491:33
Error:   4) [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom
    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29
Error:   4) [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29
Error:   4) [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeTruthy()

    Received: false

      15 |     test('should return all provider types including built-in and custom', async ({ request }) => {
      16 |       const response = await request.get('/api/v1/dns-providers/types');
    > 17 |       expect(response.ok()).toBeTruthy();
         |                             ^
      18 |
      19 |       const data = await response.json();
      20 |       // API returns { types: [...], total: N }
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:17:29
Error:   5) [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields
    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30
Error:   5) [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30
Error:   5) [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: types is not iterable

      39 |       const types = data.types;
      40 |
    > 41 |       for (const provider of types) {
         |                              ^
      42 |         expect(provider).toHaveProperty('type');
      43 |         expect(provider).toHaveProperty('name');
      44 |         expect(provider).toHaveProperty('fields');
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:41:30
Error:   6) [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration
    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36
Error:   6) [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36
Error:   6) [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      52 |       const types = data.types;
      53 |
    > 54 |       const manualProvider = types.find((t: { type: string }) => t.type === 'manual');
         |                                    ^
      55 |       expect(manualProvider).toBeDefined();
      56 |       expect(manualProvider.name).toMatch(/manual/i);
      57 |
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:54:36
Error:   7) [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field
    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37
Error:   7) [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37
Error:   7) [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      65 |       const types = data.types;
      66 |
    > 67 |       const webhookProvider = types.find((t: { type: string }) => t.type === 'webhook');
         |                                     ^
      68 |       expect(webhookProvider).toBeDefined();
      69 |
      70 |       // Webhook should have URL configuration field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:67:37
Error:   8) [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields
    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37
Error:   8) [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37
Error:   8) [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      78 |       const types = data.types;
      79 |
    > 80 |       const rfc2136Provider = types.find((t: { type: string }) => t.type === 'rfc2136');
         |                                     ^
      81 |       expect(rfc2136Provider).toBeDefined();
      82 |
      83 |       // RFC2136 (Dynamic DNS) should have server and TSIG key fields
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:80:37
Error:   9) [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field
    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36
Error:   9) [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36
Error:   9) [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    TypeError: Cannot read properties of undefined (reading 'find')

      91 |       const types = data.types;
      92 |
    > 93 |       const scriptProvider = types.find((t: { type: string }) => t.type === 'script');
         |                                    ^
      94 |       expect(scriptProvider).toBeDefined();
      95 |
      96 |       // Script provider should have a command or script path field
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:93:36
Error:   10) [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search › Verify options can be navigated with keyboard
    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7
Error:   10) [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search › Verify options can be navigated with keyboard

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7
Error:   10) [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search › Verify options can be navigated with keyboard

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(received).toBeGreaterThan(expected)

    Expected: > 5
    Received:   1

      165 |
      166 |         // Should have multiple provider options available
    > 167 |         expect(optionCount).toBeGreaterThan(5);
          |                             ^
      168 |
      169 |         // Verify cloudflare option exists in the list
      170 |         await expect(page.getByRole('option', { name: /cloudflare/i })).toBeVisible();
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:167:29
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:158:7
Error:   11) [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected › Verify Script path/command field appears
    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18
Error:   11) [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected › Verify Script path/command field appears

    Retry #1 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18
Error:   11) [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected › Verify Script path/command field appears

    Retry #2 ───────────────────────────────────────────────────────────────────────────────────────
    Error: expect(locator).toBeVisible() failed

    Locator: getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))
    Expected: visible
    Timeout: 5000ms
    Error: element(s) not found

    Call log:
      - Expect "toBeVisible" with timeout 5000ms
      - waiting for getByRole('textbox', { name: /script path/i }).or(getByPlaceholder(/dns-challenge\.sh/i))


      262 |         const scriptField = page.getByRole('textbox', { name: /script path/i })
      263 |           .or(page.getByPlaceholder(/dns-challenge\.sh/i));
    > 264 |         await expect(scriptField).toBeVisible();
          |                                   ^
      265 |       });
      266 |     });
      267 |   });
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:264:35
        at /home/runner/work/Charon/Charon/tests/dns-provider-types.spec.ts:260:18
Notice:   11 failed
    [chromium] › tests/dns-provider-crud.spec.ts:16:5 › DNS Provider CRUD Operations › Create Provider › should create a Manual DNS provider
    [chromium] › tests/dns-provider-crud.spec.ts:501:5 › DNS Provider CRUD Operations › API Operations › should list providers via API
    [chromium] › tests/dns-provider-crud.spec.ts:534:5 › DNS Provider CRUD Operations › API Operations › should reject invalid provider type via API
    [chromium] › tests/dns-provider-types.spec.ts:15:5 › DNS Provider Types › API: /api/v1/dns-providers/types › should return all provider types including built-in and custom
    [chromium] › tests/dns-provider-types.spec.ts:36:5 › DNS Provider Types › API: /api/v1/dns-providers/types › each provider type should have required fields
    [chromium] › tests/dns-provider-types.spec.ts:49:5 › DNS Provider Types › API: /api/v1/dns-providers/types › manual provider type should have correct configuration
    [chromium] › tests/dns-provider-types.spec.ts:62:5 › DNS Provider Types › API: /api/v1/dns-providers/types › webhook provider type should have url field
    [chromium] › tests/dns-provider-types.spec.ts:75:5 › DNS Provider Types › API: /api/v1/dns-providers/types › rfc2136 provider type should have server and key fields
    [chromium] › tests/dns-provider-types.spec.ts:88:5 › DNS Provider Types › API: /api/v1/dns-providers/types › script provider type should have command/path field
    [chromium] › tests/dns-provider-types.spec.ts:153:5 › DNS Provider Types › UI: Provider Selector › should filter provider types based on search
    [chromium] › tests/dns-provider-types.spec.ts:278:5 › DNS Provider Types › Provider Type Selection › should show script path field when Script type is selected
  4 skipped
  40 passed (2.0m)
Error: Process completed with exit code 1.
