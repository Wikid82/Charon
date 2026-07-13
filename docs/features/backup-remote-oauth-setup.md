---
title: Connecting Dropbox or Google Drive
description: One-time setup for admins before Dropbox or Google Drive can be used as remote backup storage
---

# Connecting Dropbox or Google Drive

Dropbox and Google Drive are the two remote storage options that use a **"Connect"** button instead of typing in a password — you sign in to Dropbox or Google directly, approve access, and Charon never sees your Dropbox/Google password at all. This is called OAuth, and it's the same kind of "Sign in with Google" button you've seen on other websites.

The trade-off: because Charon is self-hosted (there's no single shared "Charon" app that every user's install talks to), **each Charon instance's admin has to register their own mini "app"** with Dropbox and/or Google before the Connect button will work. It sounds intimidating but it's a five-minute, one-time form — this guide walks through it.

If you just want S3, SFTP, or WebDAV remote storage (no sign-in step, just host + credentials), see [Backup & Restore: Remote Storage](backup-restore.md#remote-storage) instead — you can skip this whole page.

## Before You Start: Set Your Application URL

Both Dropbox and Google need to know where to send you back after you approve access. Charon builds that address from the **Application URL** you already configure for user invitation emails:

1. Go to **System Settings** (gear icon in sidebar)
2. Scroll to the **Application URL** section
3. Enter the URL people actually use to reach your Charon instance (e.g. `https://charon.example.com`) — see [Getting Started: Configure Application URL](../getting-started.md#step-2-configure-application-url-recommended) for the full walkthrough
4. Click **Validate**, then **Save Changes**

If this isn't set, clicking **Connect** on a Dropbox or Google Drive target will show an error telling you to configure it first. No restart is needed — as soon as you save the Application URL, **Connect** will work.

**This value must match, exactly, what you register with Dropbox/Google below** (same scheme, same domain, no trailing path). A mismatch is the single most common reason "Connect" fails.

## Connecting Dropbox

### 1. Register an app in the Dropbox App Console

1. Go to the [Dropbox App Console](https://www.dropbox.com/developers/apps) and sign in
2. Click **Create app**
3. Choose **Scoped access**
4. Choose an access type:
   - **App folder** (recommended) — Charon can only see a single dedicated folder it creates in the user's Dropbox, nothing else
   - **Full Dropbox** — Charon can see everything in the account; only use this if you specifically need it
5. Give the app a name (e.g. "Charon Backups") and create it
6. On the app's **Settings** tab, note the **App key** and **App secret** — you'll paste these into Charon
7. Still on **Settings**, find **Redirect URIs** and add:

   ```
   https://your-application-url/api/v1/backups/remote-targets/oauth/dropbox/callback
   ```

   replacing `https://your-application-url` with the exact Application URL from the step above

8. Under the **Permissions** tab, make sure `files.content.write` and `files.content.read` are checked, then save

### 2. Add the Dropbox target in Charon

1. Go to **Tasks** → **Backups** → **Remote Storage Targets** → **Add Remote Target**
2. Set **Storage Type** to **Dropbox**
3. Fill in **App Key** and **App Secret** from step 1, and optionally a **Folder Path** (e.g. `/charon-backups`)
4. Click **Save & Connect** — you'll be sent to Dropbox to sign in and approve access, then brought back to Charon automatically

The target's badge will read **Connected** once this succeeds.

## Connecting Google Drive

### 1. Register a project in Google Cloud Console

1. Go to the [Google Cloud Console](https://console.cloud.google.com/) and create a new project (or pick an existing one you're comfortable using)
2. Open **APIs & Services** → **Library**, search for **Google Drive API**, and click **Enable**
3. Open **APIs & Services** → **OAuth consent screen**
   - Choose **External** (unless you have a Google Workspace organization and prefer **Internal**)
   - Fill in the required app name/support email fields
   - Add your own Google account under **Test users** if the app stays in "Testing" publishing status (this avoids Google's app-verification review, which isn't needed for a personal self-hosted setup)
4. Open **APIs & Services** → **Credentials** → **Create Credentials** → **OAuth client ID**
   - Application type: **Web application**
   - Under **Authorized redirect URIs**, add:

     ```
     https://your-application-url/api/v1/backups/remote-targets/oauth/google_drive/callback
     ```

     replacing `https://your-application-url` with the exact Application URL from the step above
5. Note the **Client ID** and **Client Secret** shown after creation — you'll paste these into Charon

### 2. Add the Google Drive target in Charon

1. Go to **Tasks** → **Backups** → **Remote Storage Targets** → **Add Remote Target**
2. Set **Storage Type** to **Google Drive**
3. Fill in **Client ID** and **Client Secret** from step 1, and optionally a **Folder Path** (e.g. `Charon/Backups`) — Charon creates any missing folders in the chain automatically
4. Click **Save & Connect** — you'll be sent to Google to sign in and approve access, then brought back to Charon automatically

The target's badge will read **Connected** once this succeeds.

## What's Not Supported (Yet)

- **Personal accounts only.** Dropbox Business/Team-space folders and Google Shared Drives aren't supported — only a personal Dropbox account or "My Drive" in a personal Google account.
- **No Digest auth for WebDAV.** The WebDAV option (see [Backup & Restore: Remote Storage](backup-restore.md#remote-storage)) supports a username/password (Basic auth) or a bearer token, which covers common self-hosted WebDAV servers like Nextcloud, ownCloud, and generic Apache/nginx WebDAV folders. Servers that require Digest auth aren't supported.

## Reconnecting

If a target's badge shows **Revoked** — usually because access was removed from inside your Dropbox or Google account settings, not from Charon — click **Reconnect** on that target's row and approve access again. Scheduled backups to that target fail (and are recorded as failed on the backup itself) until you reconnect; they don't stop your local backups from running.

## Related

- [Backup & Restore](backup-restore.md) — the full backup, restore, and remote storage guide
- [Getting Started: Configure Application URL](../getting-started.md#step-2-configure-application-url-recommended)
- [Back to Features](../features.md)
