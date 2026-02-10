diff --git a/.github/workflows/e2e-tests-split.yml b/.github/workflows/e2e-tests-split.yml
index efbcccda..64fcc121 100644
--- a/.github/workflows/e2e-tests-split.yml
+++ b/.github/workflows/e2e-tests-split.yml
@@ -213,7 +213,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'chromium' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 40
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -330,6 +330,7 @@ jobs:

           npx playwright test \
             --project=chromium \
+            --output=playwright-output/security-chromium \
             tests/security-enforcement/ \
             tests/security/ \
             tests/integration/multi-feature-workflows.spec.ts || STATUS=$?
@@ -370,6 +371,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: e2e-diagnostics-chromium-security
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
@@ -394,7 +414,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'firefox' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 40
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -519,6 +539,7 @@ jobs:

           npx playwright test \
             --project=firefox \
+            --output=playwright-output/security-firefox \
             tests/security-enforcement/ \
             tests/security/ \
             tests/integration/multi-feature-workflows.spec.ts || STATUS=$?
@@ -559,6 +580,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: e2e-diagnostics-firefox-security
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
@@ -583,7 +623,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'webkit' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 40
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -708,6 +748,7 @@ jobs:

           npx playwright test \
             --project=webkit \
+            --output=playwright-output/security-webkit \
             tests/security-enforcement/ \
             tests/security/ \
             tests/integration/multi-feature-workflows.spec.ts || STATUS=$?
@@ -748,6 +789,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: e2e-diagnostics-webkit-security
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
@@ -779,7 +839,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'chromium' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'non-security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 30
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -885,6 +945,7 @@ jobs:
           npx playwright test \
             --project=chromium \
             --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
+            --output=playwright-output/chromium-shard-${{ matrix.shard }} \
             tests/core \
             tests/dns-provider-crud.spec.ts \
             tests/dns-provider-types.spec.ts \
@@ -915,6 +976,14 @@ jobs:
           path: playwright-report/
           retention-days: 14

+      - name: Upload Playwright output (Chromium shard ${{ matrix.shard }})
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: playwright-output-chromium-shard-${{ matrix.shard }}
+          path: playwright-output/chromium-shard-${{ matrix.shard }}/
+          retention-days: 7
+
       - name: Upload Chromium coverage (if enabled)
         if: always() && (inputs.playwright_coverage == 'true' || vars.PLAYWRIGHT_COVERAGE == '1')
         uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
@@ -931,6 +1000,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: e2e-diagnostics-chromium-shard-${{ matrix.shard }}
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
@@ -955,7 +1043,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'firefox' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'non-security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 30
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -1069,6 +1157,7 @@ jobs:
           npx playwright test \
             --project=firefox \
             --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
+            --output=playwright-output/firefox-shard-${{ matrix.shard }} \
             tests/core \
             tests/dns-provider-crud.spec.ts \
             tests/dns-provider-types.spec.ts \
@@ -1099,6 +1188,14 @@ jobs:
           path: playwright-report/
           retention-days: 14

+      - name: Upload Playwright output (Firefox shard ${{ matrix.shard }})
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: playwright-output-firefox-shard-${{ matrix.shard }}
+          path: playwright-output/firefox-shard-${{ matrix.shard }}/
+          retention-days: 7
+
       - name: Upload Firefox coverage (if enabled)
         if: always() && (inputs.playwright_coverage == 'true' || vars.PLAYWRIGHT_COVERAGE == '1')
         uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
@@ -1115,6 +1212,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
+        with:
+          name: e2e-diagnostics-firefox-shard-${{ matrix.shard }}
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
@@ -1139,7 +1255,7 @@ jobs:
     if: |
       ((inputs.browser || 'all') == 'webkit' || (inputs.browser || 'all') == 'all') &&
       ((inputs.test_category || 'all') == 'non-security' || (inputs.test_category || 'all') == 'all')
-    timeout-minutes: 30
+    timeout-minutes: 60
     env:
       CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
       CHARON_EMERGENCY_SERVER_ENABLED: "true"
@@ -1253,6 +1369,7 @@ jobs:
           npx playwright test \
             --project=webkit \
             --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
+            --output=playwright-output/webkit-shard-${{ matrix.shard }} \
             tests/core \
             tests/dns-provider-crud.spec.ts \
             tests/dns-provider-types.spec.ts \
@@ -1283,6 +1400,14 @@ jobs:
           path: playwright-report/
           retention-days: 14

+      - name: Upload Playwright output (WebKit shard ${{ matrix.shard }})
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
+        with:
+          name: playwright-output-webkit-shard-${{ matrix.shard }}
+          path: playwright-output/webkit-shard-${{ matrix.shard }}/
+          retention-days: 7
+
       - name: Upload WebKit coverage (if enabled)
         if: always() && (inputs.playwright_coverage == 'true' || vars.PLAYWRIGHT_COVERAGE == '1')
         uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
@@ -1299,6 +1424,25 @@ jobs:
           path: test-results/**/*.zip
           retention-days: 7

+      - name: Collect diagnostics
+        if: always()
+        run: |
+          mkdir -p diagnostics
+          uptime > diagnostics/uptime.txt
+          free -m > diagnostics/free-m.txt
+          df -h > diagnostics/df-h.txt
+          ps aux > diagnostics/ps-aux.txt
+          docker ps -a > diagnostics/docker-ps.txt || true
+          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
+
+      - name: Upload diagnostics
+        if: always()
+        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
+        with:
+          name: e2e-diagnostics-webkit-shard-${{ matrix.shard }}
+          path: diagnostics/
+          retention-days: 7
+
       - name: Collect Docker logs on failure
         if: failure()
         run: |
