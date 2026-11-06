---
title: "Manual Test Plan: 'What's New' Changelog Modal"
labels:
  - testing
  - feature
  - frontend
priority: medium
milestone: ""
assignees: []
---

# Manual Test Plan: "What's New" Changelog Modal

## Description

Manual, human-driven checklist for the "What's New" changelog popup that
greets users after an app update. It is run against real, CI-built Charon
images, not fixture data. Automated E2E tests cover the UI on an unversioned
`dev` image. This plan covers what those tests cannot: real version
numbers, real upgrades on a persistent data volume, and real login sessions.
The design is in `docs/superpowers/specs/2026-07-28-whats-new-changelog-design.md`.

Each image reports its version on the health endpoint. Expected values are in
`VERSION.md` under "Build Versions".

## Prerequisites

- A host with Docker. A rootless Docker daemon is recommended. If your daemon
  is not the default one, set it first, for example
  `export DOCKER_HOST=unix:///run/user/<uid>/docker.sock`.
- `curl`, `jq` and a checkout of this repository (for
  `scripts/ci/smoke-whats-new.sh`).
- The images to test, by tag:
  - a pull request image, `ghcr.io/wikid82/charon:pr-<N>`
  - the published stable image, `ghcr.io/wikid82/charon:latest`
  - optionally, a release image, `ghcr.io/wikid82/charon:<X.Y.Z>`
- A browser for the UI steps (the Playwright MCP browser is fine).
- Never run these steps against a live instance you depend on. Every container
  and volume below uses a `qa-wn-` or `smoke-wn-` name, so existing containers
  and volumes are never touched.
- A local dev override exists (`CHARON_CHANGELOG_VERSION`, honored only when
  `CHARON_ENV` is not `production`). It is for UI iteration only. Do not use it
  as evidence for this plan.

## Test Cases

### First Login After a Version Bump

- [ ] Set a test user's `last_seen_version` to an older version than the
      configured "current" version, then log in — the "What's New" popup
      appears automatically after the page loads (not before — login
      itself is never delayed or blocked waiting on this).
- [ ] Confirm the popup lists the expected new features and fixes for the
      versions between the user's old `last_seen_version` and the current
      one, newest version first.
- [ ] Confirm routine maintenance items (dependency bumps, refactors, etc.)
      are tucked behind a collapsed "Show maintenance details" section, not
      shown by default.
- [ ] Log in again with the same user (without dismissing) — confirm the
      popup still appears every time until one of the dismiss actions below
      is used.

### Dismiss Path 1 — "Remind Me Next Time"

- [ ] With the popup showing, click **"Remind Me Next Time"**.
- [ ] Popup closes immediately.
- [ ] Log out and log back in as the same user — **the popup appears
      again** (this dismissal is temporary; it does not mark the update as
      seen).

### Dismiss Path 2 — "Got It, Thanks"

- [ ] With the popup showing (fresh test user or reset `last_seen_version`
      again), click **"Got It, Thanks"**.
- [ ] Popup closes immediately.
- [ ] Log out and log back in as the same user — **the popup does NOT
      appear again** for this version (this dismissal permanently records
      the current version as seen).
- [ ] Confirm this holds across multiple subsequent logins, not just the
      next one.

### Dismiss Path 3 — Close Button (X) / Backdrop Click

- [ ] With the popup showing (reset `last_seen_version` again), click the
      **X** icon in the corner — popup closes.
- [ ] Log out/in again — popup **reappears** (X behaves the same as
      "Remind Me Next Time": a temporary snooze, not a permanent dismissal).
- [ ] Repeat, this time clicking outside the popup on the dark backdrop
      instead of the X — same result: popup closes, and reappears on next
      login.

### Opt-Out Checkbox

- [ ] With the popup showing, check **"Don't show me update notifications"**
      and then click **"Remind Me Next Time"** (a temporary-dismiss path).
- [ ] Log out and log back in — confirm the popup does **NOT** appear, even
      though this dismiss path alone would normally bring it back. (The
      opt-out checkbox is independent of which button was clicked.)
- [ ] Repeat this check pairing the checkbox with **"Got It, Thanks"** and
      with the **X**/backdrop close — in all three cases, checking the box
      suppresses all future automatic popups regardless of which close
      method was used.

### Re-Enabling via Appearance Settings

- [ ] As a user who is currently opted out (from the previous section),
      navigate to **Appearance Settings**.
- [ ] Confirm the **"Show update notifications"** toggle reflects the
      opted-out state (off).
- [ ] Turn the toggle on.
- [ ] Log out and log back in with a `last_seen_version` older than
      current — confirm the popup **now appears again** (opt-out has been
      cleared, and this action does not by itself mark anything as seen).
- [ ] Turn the toggle off again directly from Appearance Settings (without
      going through the popup) — confirm this also suppresses future
      automatic popups, matching the checkbox behavior above.

### "What's New" Revisit Link (Browse Mode)

- [ ] From Appearance Settings, click the **"What's New"** link/button.
- [ ] Confirm the same modal opens, showing the **full** changelog history
      (not just entries since `last_seen_version`).
- [ ] Confirm this browse-mode view only offers a single **Close** action —
      no "Remind Me"/"Got It, Thanks" buttons, no opt-out checkbox.
- [ ] While the list loads, a loading message is shown. If the list cannot
      load, an error message is shown instead of an empty dialog.
- [ ] Confirm the first 10 releases are shown, newest first. If there are
      more, a **"Show older releases"** button appears. Each click adds the
      next 10. Reopening the dialog starts again at 10.
- [ ] Close it, then check the test user's `last_seen_version` and opt-out
      state directly in the database — confirm **neither changed**.
      Browsing must have zero side effects.
- [ ] Use this link while already opted out — confirm it still opens and
      shows full history (browsing works regardless of opt-out status).

### Built-Image Matrix (automated smoke check)

The smoke script starts an image on a temporary volume, reads its version,
creates a user, and checks the What's New API and modal rules. CI runs the
same script before every publish. Run it by hand against a pull request image:

```bash
bash scripts/ci/smoke-whats-new.sh --image ghcr.io/wikid82/charon:pr-<N> \
  --expect-version-exact '0.46.5+pr.<N>' --expect-min-entries 100 \
  --expect-newest 0.46.5 --expect-status-for-unseeded shown
```

Use `--expect-version-regex` instead of `--expect-version-exact` when you
want a pattern. In a regex, the `+` must be escaped, as in `0\.46\.5\+dev`.
The script exits `0` when every check passes. It removes everything it
created, even on failure.

| Image | Version flag | Other flags | What must be true |
| --- | --- | --- | --- |
| Pull request `pr-<N>` | `--expect-version-exact '<X.Y.Z>+pr.<N>'` | `--expect-newest <X.Y.Z>` | At least 100 entries, newest is `X.Y.Z`, no duplicates, no merge-commit subjects |
| Development `dev` | `--expect-version-exact '<X.Y.Z>+dev.<sha7>'` | `--expect-newest <X.Y.Z>` | Same as above |
| Nightly `nightly` | `--expect-version-exact '<X.Y.Z>+nightly.<sha7>'` | `--expect-commit <full 40-character commit>` | The commit matches the nightly job's built commit, and every entry is at or below `X.Y.Z` |
| Non-release `main` | `--expect-version-exact '<X.Y.Z>'` (no label) | `--expect-newest <X.Y.Z>` | Every entry is at or below `X.Y.Z` |
| Release `X.Y.Z` | `--expect-version-exact '<X.Y.Z>'` | `--expect-newest <X.Y.Z>` | The newest entry is the release itself |

Every row also takes `--expect-min-entries 100 --expect-status-for-unseeded shown`.
These check the new-user rule: the popup shows exactly 3 entries, and after
"Got it" it stays hidden on the next two logins. The CI smoke check uses the
same flags.

### Upgrade Path (real data volume)

This checks what users see after moving from one image to another while
keeping their data. Use a new volume for each run and delete it afterwards.

1. Pick a unique name, for example `qa-wn-1234-data`. Create the volume and run
   the published stable image on it, mapping its port to your own machine only:

   ```bash
   docker volume create qa-wn-1234-data
   docker run -d --name qa-wn-1234-old -p 127.0.0.1:18080:8080 \
     -v qa-wn-1234-data:/app/data ghcr.io/wikid82/charon:latest
   ```

2. Open `http://127.0.0.1:18080`, finish setup, and create three users: **A**,
   **B** and **C**. Each new user is seeded with the running version (`0.46.5` here).
3. Make B look like an account seeded long ago, and C look like an account
   from before the feature existed. The database is inside the container:

   ```bash
   docker exec qa-wn-1234-old sqlite3 /app/data/charon.db \
     "UPDATE users SET last_seen_version='0.36.1' WHERE email='<B email>';
      UPDATE users SET last_seen_version='' WHERE email='<C email>';"
   ```

4. Stop and remove the old container. Keep the volume:

   ```bash
   docker rm -f qa-wn-1234-old
   ```

5. Start the image under test on the same volume. Use a pull request image
   or a release image. Map the port in the same way:

   ```bash
   docker run -d --name qa-wn-1234-new -p 127.0.0.1:18080:8080 \
     -v qa-wn-1234-data:/app/data ghcr.io/wikid82/charon:pr-<N>
   ```

6. Log in as each user and check:
   - [ ] **A** sees no popup on the pull request image, because A's stored version equals the running base version.
   - [ ] **B** sees exactly 3 entries, and all are at or below the running base version. A stored `0.36.1` must not produce about 35 entries.
   - [ ] **C** sees exactly 3 entries, newest first.
   - [ ] After **"Got It, Thanks"**, B and C see no popup on the next two logins.

7. Downgrade check. This proves that an older image never lowers a stored
   version. Keep the same `DOCKER_HOST` from Prerequisites, if you set one.

   a. While the image under test is still running, write down the stored
      versions:

      ```bash
      docker exec qa-wn-1234-new sqlite3 /app/data/charon.db \
        "SELECT email, last_seen_version FROM users;"
      ```

   b. Stop that container. Keep it for now, so step 8 can remove it:

      ```bash
      docker stop qa-wn-1234-new
      ```

   c. Start an older image on the same volume. It must report a lower version
      than the image under test. Check with
      `curl -s http://127.0.0.1:18080/api/v1/health` and look at the `version`
      field. The stable image is used in this example:

      ```bash
      docker run -d --name qa-wn-1234-downgrade -p 127.0.0.1:18080:8080 \
        -v qa-wn-1234-data:/app/data ghcr.io/wikid82/charon:latest
      ```

   d. Sign in as **B** through the API, then dismiss the popup permanently. The
      popup stays hidden on the older image, so the browser cannot send this
      request. Use the same endpoints as the CI smoke check
      (`scripts/ci/smoke-whats-new.sh`):

      ```bash
      TOKEN=$(curl -sS -X POST http://127.0.0.1:18080/api/v1/auth/login \
        -H 'Content-Type: application/json' \
        -d '{"email":"<B email>","password":"<B password>"}' | jq -r '.token')

      curl -sS -o /dev/null -w '%{http_code}\n' -X POST \
        http://127.0.0.1:18080/api/v1/changelog/ack \
        -H "Authorization: Bearer $TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"action":"dismiss_permanent"}'
      ```

      The last line must print a `2xx` code, such as `200`.

   e. Read the stored versions from the older container:

      ```bash
      docker exec qa-wn-1234-downgrade sqlite3 /app/data/charon.db \
        "SELECT email, last_seen_version FROM users;"
      ```

   - [ ] **B**'s `last_seen_version` is the same as, or higher than, the value
         you wrote down in step a. It must not go lower.
   - [ ] No other user's value went lower either.

8. Clean up. Remove the containers and the volume you created:

   ```bash
   docker rm -f qa-wn-1234-new qa-wn-1234-downgrade
   docker volume rm qa-wn-1234-data
   ```

For a release upgrade, use the previous release as the first image and the
new release as the second. Expected results: users see only versions after
their stored version, up to the new release, and never more than 3 entries.

### Browser Checks on a Built Image

Start a throwaway container from the image under test
(`docker run -d --name qa-wn-1234-ui -p 127.0.0.1:18080:8080 ...`) and remove
it afterwards.

- [ ] **Passthrough users are blocked.** Create a passthrough user, log in as
      that user, and call the four What's New API routes (`/api/v1/changelog/status`,
      `/all`, `/ack`, `/opt-in`). Each returns `403`. The modal does not open.
- [ ] **Revisit dialog.** Open Settings > Appearance > What's New. The dialog
      shows a loading state, then the newest 10 releases with "Show older releases".
      Headings do not start with emoji in their accessible name. Check the
      browser console: no React key warnings appear.
- [ ] **System Settings update card.** Open Settings > System. The card shows the
      running version (matching `/api/v1/health`). "Check for updates" triggers a
      fetch and shows the status. A release link appears only when it is an
      `https://github.com` address.

## Notes

- The popup must never delay or block the login/dashboard flow — if the
  changelog check is slow or fails outright, the app should load normally
  with no popup and no visible error.
- Brand-new users (created after this feature shipped) should never see
  historical changelog entries on their first login — their
  `last_seen_version` is seeded at account creation. Worth a quick sanity
  check if a fresh invite/admin-created account is available during this
  pass.
- An unversioned `dev` build never shows the automatic popup. Its Revisit list
  shows whatever data it embeds, which is empty in a local build.
