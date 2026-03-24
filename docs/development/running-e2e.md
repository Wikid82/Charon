# Running Playwright E2E (headed and headless)

This document explains how to run Playwright tests using a real browser (headed) on Linux machines and in the project's Docker E2E environment.

## Key points

- Playwright's interactive Test UI (--ui) requires an X server (a display). On headless CI or servers, use Xvfb.
- Prefer the project's E2E Docker image for integration-like runs; use the local `--ui` flow for manual debugging.

## Quick commands (local Linux)

- Headless (recommended for CI / fast runs):

  ```bash
  npm run e2e
  ```

- Headed UI on a headless machine (auto-starts Xvfb):

  ```bash
  npm run e2e:ui:headless-server
  # or, if you prefer manual control:
  xvfb-run --auto-servernum --server-args='-screen 0 1280x720x24' npx playwright test --ui
  ```

- Headed UI on a workstation with an X server already running:

  ```bash
  npx playwright test --ui
  ```

- Open the running Docker E2E app in your system browser (one-step via VS Code task):
  - Run the VS Code task: **Open: App in System Browser (Docker E2E)**
  - This will rebuild the E2E container (if needed), wait for <http://localhost:8080> to respond, and open your system browser automatically.

- Open the running Docker E2E app in VS Code Simple Browser:
  - Run the VS Code task: **Open: App in Simple Browser (Docker E2E)**
  - Then use the command palette: `Simple Browser: Open URL` → paste `http://localhost:8080`

## Using the project's E2E Docker image (recommended for parity with CI)

1. Rebuild/start the E2E container (this sets up the full test environment):

   ```bash
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e
   ```

  If you need a clean rebuild after integration alignment changes:

  ```bash
  .github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean --no-cache
  ```

1. Run the UI against the container (you still need an X server on your host):

   ```bash
   PLAYWRIGHT_BASE_URL=http://localhost:8080 npm run e2e:ui:headless-server
   ```

## CI guidance

- Do not run Playwright `--ui` in CI. Use headless runs or the E2E Docker image and collect traces/videos for failures.
- For coverage, use the provided skill: `.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage`

## Troubleshooting

- Playwright error: "Looks like you launched a headed browser without having a XServer running." → run `npm run e2e:ui:headless-server` or install Xvfb.
- If `npm run e2e:ui:headless-server` fails with an exit code like `148`:
  - Inspect Xvfb logs: `tail -n 200 /tmp/xvfb.playwright.log`
  - Ensure no permission issues on `/tmp/.X11-unix`: `ls -la /tmp/.X11-unix`
  - Try starting Xvfb manually: `Xvfb :99 -screen 0 1280x720x24 &` then `export DISPLAY=:99` and re-run `npx playwright test --ui`.
- If running inside Docker, prefer the skill-runner which provisions the required services; the UI still needs host X (or use VNC).

## Developer notes (what we changed)

- Added `scripts/run-e2e-ui.sh` — wrapper that auto-starts Xvfb when DISPLAY is unset.
- Added `npm run e2e:ui:headless-server` to run the Playwright UI on headless machines.
- Playwright config now auto-starts Xvfb when `--ui` is requested locally and prints an actionable error if Xvfb is not available.

## Security & hygiene

- Playwright auth artifacts are ignored by git (`playwright/.auth/`). Do not commit credentials.

---
If you'd like, I can open a PR with these changes (scripts + config + docs) and add a short CI note to `.github/` workflows.
