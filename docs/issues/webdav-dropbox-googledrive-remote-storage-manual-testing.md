---
title: "Manual Testing: WebDAV / Dropbox / Google Drive Remote Backup Storage"
labels:
  - testing
  - feature
  - frontend
  - backend
  - security
priority: medium
milestone: "v0.4.0"
assignees: []
---

# Manual Testing: WebDAV / Dropbox / Google Drive Remote Backup Storage

## Description

Manual testing plan for the three new remote backup storage provider types
added alongside the existing S3/SFTP options: generic WebDAV, Dropbox, and
Google Drive. Covers real-provider OAuth flows, SSRF protection against a
live self-hosted WebDAV server, and retention/pagination behavior that
automated tests exercise against fakes but should also be verified against
real vendor APIs at least once before this ships to users. See
`docs/plans/current_spec.md` for the full technical spec and
`docs/reports/qa_report.md` for the automated Definition-of-Done audit this
plan supplements (not replaces).

## Prerequisites

- A self-hosted WebDAV server reachable from the Charon instance (Nextcloud,
  ownCloud, or generic Apache/nginx `mod_dav`), with a dedicated test folder.
- A Dropbox account with a registered App Console app (App Folder or Full
  Dropbox scope) — App Key + App Secret, redirect URI registered.
- A Google Cloud project with the Drive API enabled, an OAuth consent screen
  configured, and OAuth 2.0 credentials (Client ID + Client Secret) with the
  redirect URI registered.
- Charon's **Application URL** (System Settings → `app.public_url`) set to a
  publicly reachable HTTPS origin that matches what's registered in both the
  Dropbox App Console and the Google Cloud OAuth client.
- `CHARON_ENCRYPTION_KEY` configured (remote-target secret storage requires
  it, same as existing S3/SFTP targets).

## Test Cases

### WebDAV — UI/UX and CRUD

- [ ] Select "WebDAV" from the target type options — URL, username, base
      path, "skip TLS verification" checkbox, and password fields appear
- [ ] Create a WebDAV target against the real test server — succeeds, shows
      a "never tested" badge until Test is clicked
- [ ] Click Test — shows success against the real server
- [ ] Edit the target, leave password blank — existing credential is
      preserved (confirm a subsequent Test still succeeds)
- [ ] Edit the target, enter a new password — old credential is replaced
      (confirm with a deliberately wrong new password that Test now fails)
- [ ] Delete the target — removed from the list, no orphaned data

### WebDAV — Backup Upload/Retention (Real Server)

- [ ] Trigger a manual backup to the WebDAV target — file appears in the
      configured base path on the real server
- [ ] Trigger enough backups to exceed the configured retention count —
      oldest backup(s) are deleted from the real server, newest ones remain
- [ ] Point the base path at a nested, not-yet-existing folder — folder
      chain is created automatically on first upload

### WebDAV — SSRF Protection (Real Network)

- [ ] Attempt to create a WebDAV target with URL host `127.0.0.1` or
      `localhost` — rejected at save time with a clear error
- [ ] Attempt a link-local address (`169.254.169.254`) — rejected at save
      time
- [ ] Attempt a private RFC1918 address that's NOT the real test server
      (e.g. an unused `192.168.x.x` address on the same LAN) — save
      succeeds (RFC1918 is allowed by design), but Test/Upload fails
      cleanly with a connection error, not a crash

### Dropbox — OAuth Connect Flow (Real Provider)

- [ ] Create a Dropbox target with real App Key + App Secret, no folder
      path — "Save & Connect" redirects to Dropbox's real consent screen
- [ ] Approve access on Dropbox's real consent screen — redirected back to
      Charon, target shows "connected" badge
- [ ] Deny access on Dropbox's consent screen — redirected back to Charon
      with an error toast, target remains "not_connected", no partial state
- [ ] Disconnect a connected Dropbox target — badge reverts to
      "not_connected", App Secret is preserved (re-Connect doesn't require
      re-entering it)
- [ ] Reconnect after disconnect — full OAuth round trip works again

### Dropbox — Backup Upload/Retention (Real Account)

- [ ] Trigger a manual backup to the connected Dropbox target — file
      appears in the configured folder path in the real Dropbox account
- [ ] Trigger enough backups to exceed retention — oldest deleted, newest
      retained (verify in the real Dropbox account, not just Charon's UI)
- [ ] If feasible, trigger an upload of a file >150MiB — verify it completes
      via the chunked upload path (check Dropbox's file version history or
      file size to confirm it wasn't truncated)
- [ ] Revoke Charon's access from Dropbox's own account security settings
      (not through Charon) — next Test/backup in Charon surfaces a
      "revoked" badge and an actionable reconnect prompt, not a generic
      failure

### Google Drive — OAuth Connect Flow (Real Provider)

- [ ] Create a Google Drive target with real Client ID + Client Secret —
      "Save & Connect" redirects to Google's real consent screen
- [ ] Approve access — redirected back to Charon, target shows "connected"
- [ ] Deny access — redirected back with an error toast, target remains
      "not_connected"
- [ ] Disconnect and reconnect — same as Dropbox above

### Google Drive — Folder Resolution and Retention (Real Account)

- [ ] Configure a multi-segment folder path (e.g. `Charon/Backups/Prod`)
      that doesn't exist yet — verify the full folder chain is created in
      the real Google Drive account on first upload
- [ ] Trigger enough backups to exceed retention — oldest file deleted by
      ID, newest retained (verify in the real Drive account)
- [ ] Manually rename or move the target folder in Google Drive between two
      scheduled backups — next backup re-resolves (and recreates if
      necessary) the folder chain rather than failing
- [ ] Manually create a file (not a folder) with the same name as a
      configured folder segment in the real Drive account — verify Charon
      still creates a separate folder alongside it rather than erroring

### Cross-Provider

- [ ] Configure one target of each type (S3, SFTP, WebDAV, Dropbox, Google
      Drive) simultaneously — all coexist, list view shows correct
      type-specific badges/controls for each
- [ ] Confirm the OAuth status badge (Dropbox/Google Drive only) never
      appears on S3/SFTP/WebDAV rows
- [ ] Confirm no browser network tab / response body ever shows
      `oauth_client_secret`, `oauth_access_token`, or `oauth_refresh_token`
      in plaintext for any target, at any point (list, create response,
      update response)

### Accessibility

- [ ] Keyboard-only navigation through the 5-way type selector and all new
      field groups (WebDAV, Dropbox, Google Drive) without focus traps
- [ ] Screen reader announces the Connect/Reconnect button and OAuth status
      badge meaningfully (not just a bare color/icon)

## Notes

- Automated E2E tests mock the OAuth provider round trip via route
  interception (real consent screens aren't reachable in CI) — this is the
  first point these flows get exercised against the *real* Dropbox and
  Google APIs, so budget extra time for provider-side surprises (rate
  limits, consent-screen verification requirements for unpublished OAuth
  apps, etc.).
- The chunked-upload-resume and WebDAV Digest-auth gaps are documented,
  accepted v1 limitations (spec §3.9, §8) — not in scope for this test pass.
- File a bug against `docs/plans/current_spec.md`'s assumptions (not just a
  code bug) if real-provider behavior contradicts anything in §3.5's
  API-shape descriptions — those were written from API documentation, not
  live-tested against the real endpoints before this pass.
