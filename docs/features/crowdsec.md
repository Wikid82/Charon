---
title: CrowdSec Integration
description: Behavior-based threat detection powered by a global community
---

# CrowdSec Integration

Protect your applications using behavior-based threat detection powered by a global community of security data. Bad actors get blocked automatically before they can cause harm.

## Overview

CrowdSec analyzes your traffic patterns and blocks malicious behavior in real-time. Unlike traditional firewalls that rely on static rules, CrowdSec uses behavioral analysis and crowdsourced threat intelligence to identify and stop attacks.

Key capabilities:

- **Behavior Detection** — Identifies attack patterns like brute-force, scanning, and exploitation
- **Community Blocklists** — Benefit from threats detected by the global CrowdSec community
- **Real-time Blocking** — Malicious IPs are blocked immediately via Caddy integration
- **Automatic Updates** — Threat intelligence updates continuously

## Why Use This

- **Proactive Defense** — Block attackers before they succeed
- **Zero False Positives** — Behavioral analysis reduces incorrect blocks
- **Community Intelligence** — Leverage data from thousands of CrowdSec users
- **GUI-Controlled** — Enable/disable directly from the UI, no environment variables needed

## Configuration

### Enabling CrowdSec

1. Navigate to **Settings → Security**
2. Toggle **CrowdSec Protection** to enabled
3. CrowdSec starts automatically and persists across container restarts

No environment variables or manual configuration required.

> **Admin only.** Enabling, disabling, and managing CrowdSec — bans, bouncer
> keys, rulesets, and configuration — requires an administrator account. Standard
> users don't see the CrowdSec controls.

### Hub Presets

Access pre-built security configurations from the CrowdSec Hub:

1. Go to **Settings → Security → Hub Presets**
2. Browse available collections (e.g., `crowdsecurity/nginx`, `crowdsecurity/http-cve`)
3. Search for specific parsers, scenarios, or collections
4. Click **Install** to add to your configuration

Hub presets are installed in two steps. First click **Pull** to download and preview the preset (a pulled preview can be shown again without downloading it again, and pulling a preset without picking a section reuses a recently fetched hub list for 10 minutes when only one section is saved on the server). Then click **Apply** to install exactly what you previewed, using the `cscli` tool that ships with Charon (it also installs anything the preset depends on). If the preview has gone out of date, Charon asks you to pull the preset again. Most installs finish in a few seconds. Large presets install in the background (see below) with a 30 minute limit; a quick install that does not finish within 120 seconds is stopped, rolled back, and your preview is kept. You can apply an already-downloaded preview without internet access if its items are already on the server; otherwise the apply fails and is rolled back, and pulling a preset you never downloaded needs internet access. If a name exists in more than one hub section (for example a parser and a collection with the same name), Charon asks you to choose which one you mean. If the hub cannot be reached, nothing is installed and Charon says so. Charon also checks each downloaded item against the hub's index to catch incomplete or inconsistent updates before keeping it; if an item does not match, Charon refuses it and asks you to try again later. See [CrowdSec troubleshooting](../troubleshooting/crowdsec.md#hub-preset-messages) if you see a message.

Popular presets include:

- **HTTP Probing** — Detect reconnaissance and scanning
- **Bad User-Agents** — Block known malicious bots
- **CVE Exploits** — Protection against known vulnerabilities

#### Large Installs Run in the Background

Big presets (hundreds of items) can take many minutes, so Charon installs them in the background instead of making you wait on a spinner.

- **Watch the progress.** A small progress row in the presets card shows what Charon is doing right now. Nothing needs your attention while it runs.
- **Leave and come back.** You can close the page or move to another one. The install keeps going, and the row is there when you return.
- **Cancel any time.** Click **Cancel** and Charon puts your CrowdSec settings back exactly as they were before the install started.
- **Other CrowdSec changes wait.** While an install runs, starting or stopping CrowdSec, importing a configuration, saving a file, changing the whitelist, enrolling in the Console and installing missing hub items are paused. Charon shows a "busy" message; try again when the install finishes.
- **If Charon restarts mid-install,** your previous settings are restored automatically and the job is shown as interrupted. Just start the install again.
- **Time limit.** An install is stopped and rolled back after 30 minutes.
- **History.** Charon keeps the newest 20 finished installs no matter how old they are, plus any that finished less than 7 days ago, so you can see how the last ones ended.
- **Exports keep working.** You can still export your configuration while an install runs, but that export can contain a partly installed hub tree. Export again after the install finishes if you want a clean copy.

#### Curated Presets

Charon also ships curated presets such as **Honeypot Friendly Defaults** and **GeoIP Enrichment** (`geoip-enrichment`, formerly `geolocation-aware`; it adds IP geolocation data to alerts and does not block by region). Curated presets are installed on the server with `cscli`, after a backup is taken. If the install fails, the backup is rolled back, and Charon shows the server's error instead of a success message. After a preset is applied (and after whitelist changes), Charon signals the CrowdSec process it manages to reload so new rules take effect. If CrowdSec is not running or the reload cannot be confirmed, a "reload required" note appears and the rules load the next time CrowdSec starts or restarts.

#### Safe Applies and Rollback

Hub presets are applied on the server too, with the same safety net: before anything changes, Charon makes a backup copy of your CrowdSec settings. If something goes wrong, your settings are put back exactly as they were, and Charon shows you the error. Your live CrowdSec folder is never moved or emptied along the way.

- The backup taken before a hub preset is applied is a light one: it does not copy CrowdSec's downloaded hub files, so it stays small and quick however large the hub grows. If the apply fails, your settings are put back and any hub files the failed apply added are removed.
- Charon keeps the 5 most recent full backups and the 10 most recent single-file backups, and quietly deletes older ones. Backups of your acquisition settings (`acquis.yaml.backup.<timestamp>`) are kept to the latest 5 as well.
- Only one change runs at a time, so two edits can't collide.
- The config file editor only accepts common text and settings files (`.yaml`, `.yml`, `.json`, `.txt`, `.conf`) up to 1 MiB, and validates file type, size and YAML. Extremely large or deeply nested YAML files are refused with an "invalid YAML" message.
- The config file editor does not list CrowdSec's account and connection credential files. They stay safely on disk; an operator can still reach them from the container shell if needed.
- An exported CrowdSec configuration is meant for moving settings, not for sharing. CrowdSec's account and connection details are not included. In the main `config.yaml` (and `config.yaml.local`), database passwords and other credentials, such as the database user and the automatic registration token, are blanked the same way. This is a best-effort list of known credential settings: any setting whose name ends in `password`, `passwd`, `secret`, `token`, `secret_key`, `apikey`, `api_key` or `private_key`, or contains `_password` or `_token`, is blanked, except settings that only point to a credential (names ending in `_path`, `_file` or `_dir`) or limit its use (such as `tokens_per_minute`). Credentials stored under other names are not recognised, so review the file before sharing it. If you import the export on the same server, the blanked values are put back from the file it replaces; on another host they stay blank and the import card lists the file as one that still needs values. In your notification files, passwords, tokens and webhook addresses are blanked and replaced with `<charon-redacted>`, while names, types and other plain settings stay readable. Settings written as `${NAME}` environment references are kept as they are, comments in notification files are not kept, and stray non-YAML files (such as `.bak` or `.txt`) in a notifications folder are left out. Database settings and anything else in the configuration are still included, so keep treating the file as private. When you import one, your running system keeps its own. Its installed hub items and runtime state stay as they are, so an import cannot remove or change them. An exported configuration no longer contains the downloaded hub data; CrowdSec downloads it again when needed. CrowdSec re-creates its local connection details on its own. If you move to a new host and use community blocklists or the CrowdSec Console, re-enable console enrollment there.
- Exports also leave out private key files: files that a notification setting points to as a key (the name is matched as written; a relative `key_path` is not resolved against another folder), files ending in `.key`, `.p12` or `.pfx`, and any file that holds a text-format (PEM or PGP) private key. Charon looks through a whole file up to 1 MiB, in overlapping 64 KiB windows, and looks only at the first 64 KiB of a larger file. It does not try to recognise binary (DER) keys by their content, so a binary key with another file name is not detected; name key files `*.key`, `*.p12` or `*.pfx`. Certificates and public keys are still included. After an export, a short note next to the export button lists the first few files that were left out (up to 50 are reported), and Charon's log records how many. If you restore the export on a different host, put those files there again. If a `config.yaml` (or `config.yaml.local`) cannot be safely blanked (it is larger than 1 MiB, is not valid YAML, or shares credential values through YAML anchors or merge keys), the export fails with the message "The CrowdSec configuration file could not be exported safely" instead of producing an archive that could not be imported again; simplify the file and export again. If you import an export on the same server, your current key files (up to 64, each up to 1 MiB) are kept and listed on the import card as kept files. Only key files that are actually missing count toward that limit, so a large folder of other files does not get in the way.
- When you import a blanked export, notifications that already exist on your system with the same type, name and target keep their current values. For any other notification file the placeholders stay, and the import card lists the files that still need values. Open each one in the config file editor and enter the real values. Importing a blanked export with an older version of Charon writes the placeholder into the file as plain text, so edit those values afterwards.
- A few limits to know about: the `format` message template is kept exactly as written (put secrets in `${NAME}` references, not in the template); the names of settings Charon does not recognize are kept, but their values are blanked; a notification folder you moved outside the data folder is not exported, and the `/etc/crowdsec` shortcut for a moved folder only works with the standard `config/` layout. Files that use YAML anchors, aliases or merge keys are never filled in from your current values, so they come back blanked and are listed on the card. The copy Charon makes of your current settings before an import (the `.backup.` folder next to your data folder) still has the original values.
- If Charon cannot safely blank a notification file, it exports that file as an empty one. If an alert profile uses a notification that only that file defined, importing the export is refused until you define the notification again.
- An exported configuration also lists which hub items (collections, parsers, scenarios and so on) were installed on the system it came from. After an import, if some of those items are not installed on this system, the CrowdSec page offers an **Install missing hub items** button. It downloads and installs them from the official CrowdSec hub. This needs an internet connection and can take several minutes. If the hub cannot be reached, nothing is installed and you can simply try again later. If some items were not reached in time, an **Install remaining** button lets you retry just those. Items that are already installed are never removed or changed by an import.
- Before changing anything, an import checks that every notification name used by your alert profiles exists in the imported configuration. If any are missing, the import is refused and the message lists the missing names.
- If your CrowdSec already keeps its settings in the `config/` folder (every normal Charon install does), an import must keep them there too. An archive with its settings at the top level is refused with a clear message and nothing changes. Export a configuration from this page, or put the files in a `config/` folder inside the archive, then try again.
- CrowdSec cannot start without `profiles.yaml`, `simulation.yaml`, `acquis.yaml` and a `patterns/` folder. If your archive leaves any of them out, Charon keeps the current ones and the import card lists which were kept. If a kept `profiles.yaml` names notifications, those notification files are kept too. If they still cannot be matched, the import is refused and nothing changes.
- Limits: a file or folder is kept only when it is missing entirely. One that is present but incomplete (for example a `patterns/` folder with a single file) is used as it is and is not repaired, so CrowdSec may fail to start. The `acquis.d/` folder is not checked. After a restart, Charon recreates `acquis.yaml` if it is missing, but not the other three.
- Installing missing hub items now reports an error if CrowdSec's list of installed items cannot be read, and says what was already installed. Trying again is safe. Configuration changes also stop waiting if you cancel or close the request.
- The export and import messages are translated into your language.
- The CrowdSec import accepts only `.tar.gz` archives and checks that the main configuration file is valid YAML before applying anything. If it isn't, Charon tells you what's wrong, for example `config.yaml is not valid YAML` or `only .tar.gz archives are supported`.
- Backups are shown by name and are stored next to the CrowdSec data folder (for example `/app/data/`). Full snapshots are named `crowdsec.backup.<timestamp>`, single-file saves `crowdsec.filebackup.<timestamp>`. Saved acquisition settings are backed up as `acquis.yaml.backup.<timestamp>` right beside the acquisition file instead. If Charon cannot keep the acquisition file's original owner, it saves the file as the Charon user. To recover by hand, stop CrowdSec, copy the backup you want back into place, and start it again.

### Console Enrollment

Connect to the CrowdSec Console for centralized management:

1. Go to **Settings → Security → Console Enrollment**
2. Enter your enrollment key from [console.crowdsec.net](https://console.crowdsec.net)
3. Click **Enroll**

The Console provides:

- Multi-instance management
- Historical attack data
- Alert notifications
- Blocklist subscriptions

### Live Decisions

View active blocks in real-time:

1. Navigate to **Security → Live Decisions**
2. See all currently blocked IPs with:
   - IP address and origin country (the country column stays hidden until country data is available)
   - Reason for block (scenario triggered)
   - Duration remaining
   - Option to manually unban

### Banning and Unbanning

You can block an address by hand from the Live Decisions page.

- **What you can ban**: a single address (IPv4 or IPv6), or a whole range such as `192.0.2.0/24`.
- **How long**: from 1 minute to 365 days. Write it like `30m`, `4h` or `1d`. The longest choice in the ban dialog is 365 days; there is no permanent ban.
- **Reason**: optional, up to 200 characters.
- **Unban**: works for single addresses and ranges, and removes only a decision with exactly the address or range you typed. Unbanning a range does not clear single addresses banned inside it, and unbanning an address does not clear a range that covers it. If nothing matches, Charon tells you instead of saying it worked. Each unban is added to the decision history.
- **Active decisions count**: the number on the dashboard uses Charon's saved bouncer key, so it shows a real number instead of "N/A".
- **Alerts**: the alerts list shows only the details the page uses. It follows the time range you pick and pages through up to the 1,000 most recent matching alerts.
- **Lists that cannot load**: if CrowdSec is slow, unavailable or returns too much data, the list shows a short note that it may be incomplete and refreshes on its own. Nothing needs to be done.
- **Top addresses**: the list shows when each address was last seen. The Country column only appears once country information exists for at least one address.
- **Dashboard totals**: unbans you make by hand are kept in the history, but they are not counted as blocked attackers in the top addresses or the summary.
- **Request size**: CrowdSec requests larger than 64 KiB are refused.

If a ban or unban fails, see [CrowdSec troubleshooting](../troubleshooting/crowdsec.md#ban-and-unban-messages).

## Automatic Startup & Persistence

CrowdSec settings are stored in Charon's database and synchronized with the Security Config:

- **On Container Start** — CrowdSec launches automatically if previously enabled
- **Configuration Sync** — Changes in the UI immediately apply to CrowdSec
- **State Persistence** — Decisions and configurations survive restarts
- **Downloaded Data Survives Recreating the Container** — CrowdSec's own database and the lists it downloads (such as blocklists and the location database) are stored in your Charon data folder. If any are missing after an upgrade, Charon re-downloads them on startup. If the CrowdSec Hub is unreachable, a restart with missing data files may take up to about two minutes longer while the refresh times out.

## Managing the CrowdSec bouncer key

The bouncer key is the password Charon's built-in firewall layer uses to talk to CrowdSec. Charon creates and registers it for you the first time CrowdSec starts, so most people never need to touch it.

**Where it lives:** Charon saves the key in a private file at `/app/data/crowdsec/bouncer_key` inside your data folder (readable only by Charon).

**Which key Charon uses (in order):**

1. **The saved key file.** If it exists and CrowdSec still accepts it, Charon uses it. This always wins.
2. **A key you set in your compose file.** Charon uses it only when there is no working saved key. Charon then saves it to the file, so it is kept after restarts.
3. **Otherwise,** Charon registers a new key with CrowdSec and saves it.

If you set a key in your compose file, use one of these variable names. Charon checks them in this order and uses the first one that is set:

1. `CHARON_SECURITY_CROWDSEC_API_KEY` (recommended)
2. `CROWDSEC_API_KEY`
3. `CROWDSEC_BOUNCER_API_KEY`
4. `CERBERUS_SECURITY_CROWDSEC_API_KEY`
5. `CPM_SECURITY_CROWDSEC_API_KEY`

**Changing the key through your compose file:** changing the variable alone does not replace a saved key that still works. To switch to the key in your variable, do both of these:

1. Change the variable in your compose file.
2. Delete the saved key file (see "Replace the key" below), or revoke the old key in CrowdSec.

**Using a single key file:** you can mount one key file into the container (for example, a single file from your computer). Charon then saves the key by writing into that file directly, and it sets the file's permissions to private (`0600`). If your system does not allow that, Charon writes the key anyway and logs a warning.

**Symlinks:** if `bouncer_key` is a shortcut (a symlink) to another file, the next save replaces the shortcut with a normal private file. The file the shortcut pointed to is not changed.

**If Charon cannot save the key:** Charon writes an error to the log, and it does not keep the key anywhere else. Blocking may not work until the key file can be written. To fix it, give Charon write access to the data folder, then restart CrowdSec.

**See it in the UI:** Open the CrowdSec page and look at the **Bouncer API Key** card. It shows a shortened preview of the key, whether it is registered, and where the file is stored.

**Replace the key:**

1. Remove the old registration (the bouncer is named `caddy-bouncer`):

   ```bash
   docker exec charon cscli bouncers delete caddy-bouncer
   ```

2. Delete the saved key file so Charon makes a fresh one:

   ```bash
   docker exec charon rm /app/data/crowdsec/bouncer_key
   ```

3. Restart Charon. It registers the bouncer again and saves the new key.

Replace `charon` with your container's name if it is different.

## Troubleshooting Console Enrollment

### Engine Shows "Offline" in Console

Your CrowdSec Console dashboard shows your engine as "Offline" even though it's running locally.

**Why this happens:**

CrowdSec sends periodic "heartbeats" to the Console to confirm it's alive. If heartbeats stop reaching the Console servers, your engine appears offline.

**Quick check:**

Run the diagnostic script to test connectivity:

```bash
./scripts/diagnose-crowdsec.sh
```

Or use the API endpoint:

```bash
curl http://localhost:8080/api/v1/cerberus/crowdsec/diagnostics/connectivity
```

**Common causes and fixes:**

| Cause | Fix |
| ------- | ----- |
| Firewall blocking outbound HTTPS | Allow connections to `api.crowdsec.net` on port 443 |
| DNS resolution failure | Verify `nslookup api.crowdsec.net` works |
| Proxy not configured | Set `HTTP_PROXY`/`HTTPS_PROXY` environment variables |
| Heartbeat service not running | Force a manual heartbeat (see below) |

**Force a manual heartbeat:**

```bash
curl -X POST http://localhost:8080/api/v1/cerberus/crowdsec/console/heartbeat
```

### Enrollment Token Expired or Invalid

**Error messages:**

- "token expired"
- "unauthorized"
- "invalid enrollment key"

**Solution:**

1. Log in to [console.crowdsec.net](https://console.crowdsec.net)
2. Navigate to **Instances → Add Instance**
3. Generate a new enrollment token
4. Paste the new token in Charon's enrollment form

Tokens expire after a set period. Always use a freshly generated token.

### LAPI Not Started / Connection Refused

**Error messages:**

- "connection refused"
- "LAPI not available"

**Why this happens:**

CrowdSec's Local API (LAPI) needs 30-60 seconds to fully start after the container launches.

**Check LAPI status:**

```bash
docker exec charon cscli lapi status
```

**If you see "connection refused":**

1. Wait 60 seconds after container start
2. Check CrowdSec is enabled in the Security dashboard
3. Try toggling CrowdSec OFF then ON again

### Already Enrolled Error

**Error message:** "instance already enrolled"

**Why this happens:**

A previous enrollment attempt succeeded but Charon's local state wasn't updated.

**Verify enrollment:**

1. Log in to [console.crowdsec.net](https://console.crowdsec.net)
2. Check **Instances** — your engine may already appear
3. If it's listed, Charon just needs to sync

**Force a re-sync:**

```bash
curl -X POST http://localhost:8080/api/v1/cerberus/crowdsec/console/heartbeat
```

### Network/Firewall Issues

**Symptom:** Enrollment hangs or times out

**Test connectivity manually:**

```bash
# Check DNS resolution
nslookup api.crowdsec.net

# Test HTTPS connectivity
curl -I https://api.crowdsec.net
```

**Required outbound connections:**

| Host | Port | Purpose |
| --- | --- | --- |
| `api.crowdsec.net` | 443 | Console API and heartbeats |
| `hub.crowdsec.net` | 443 | Hub presets download |

### "unable to init data for file" in the CrowdSec Log

**What it means:** CrowdSec looked for one of its downloaded helper lists (like a blocklist or location database) and the file was missing. In older versions this could happen after recreating the container.

**What to do:** Update Charon and restart the container. The missing files are downloaded again automatically (this needs internet access) and the message goes away. Nothing else is needed.

## Using the Diagnostic Script

The diagnostic script checks CrowdSec connectivity and configuration in one command.

**Run all diagnostics:**

```bash
./scripts/diagnose-crowdsec.sh
```

**Output as JSON (for automation):**

```bash
./scripts/diagnose-crowdsec.sh --json
```

**Use a custom data directory:**

```bash
./scripts/diagnose-crowdsec.sh --data-dir /custom/path
```

**What it checks:**

- LAPI availability and health
- CAPI (Central API) connectivity
- Console enrollment status
- Heartbeat service status
- Configuration file validity

## Diagnostic API Endpoints

Access diagnostics programmatically through these API endpoints:

| Endpoint | Method | What It Does |
| ---------- | -------- | -------------- |
| `/api/v1/cerberus/crowdsec/diagnostics/connectivity` | GET | Tests LAPI and CAPI connectivity |
| `/api/v1/cerberus/crowdsec/diagnostics/config` | GET | Validates enrollment configuration |
| `/api/v1/cerberus/crowdsec/console/heartbeat` | POST | Forces an immediate heartbeat check |

## Preset Install API

Routes live under `/api/v1/admin/crowdsec/presets/` and need an admin login.

| Endpoint | Method | What It Does |
| ---------- | -------- | -------------- |
| `.../apply` | POST | Installs a pulled preset. Send `"async": true` to start a background job; the reply is `202` with the job. Without it the install is synchronous, which is deprecated for large presets |
| `.../jobs/current` | GET | The running job, or the job that finished within the last 30 minutes (otherwise `{"job": null}`) |
| `.../jobs` | GET | Recent jobs (newest first; retention keeps the newest 20 finished jobs regardless of age, plus any younger than 7 days) |
| `.../jobs/:id` | GET | One job |
| `.../jobs/:id` | DELETE | Cancels a running job and rolls back |

- A pull returns `item_sha256`; apply accepts it back (optional) to make sure the install matches what was previewed.
- While a job runs, other CrowdSec changes answer `409` with `crowdsec_busy`.
- Failed jobs carry an `error_code`, such as `preset_cache_stale` (pull again), `hub_unreachable`, `preset_apply_timeout` or `interrupted`.
- Job views never include backup locations.

**Example: Check connectivity**

```bash
curl http://localhost:8080/api/v1/cerberus/crowdsec/diagnostics/connectivity
```

**Example response:**

```json
{
  "lapi": {
    "status": "healthy",
    "latency_ms": 12
  },
  "capi": {
    "status": "reachable",
    "latency_ms": 145
  }
}
```

## Reading the Logs

Look for these log prefixes when debugging:

| Prefix | What It Means |
| -------- | --------------- |
| `[CROWDSEC_ENROLLMENT]` | Enrollment operations (token validation, CAPI registration) |
| `[HEARTBEAT_POLLER]` | Background heartbeat service activity |
| `[CROWDSEC_STARTUP]` | LAPI initialization and startup |

**View enrollment logs:**

```bash
docker logs charon 2>&1 | grep CROWDSEC_ENROLLMENT
```

**View heartbeat activity:**

```bash
docker logs charon 2>&1 | grep HEARTBEAT_POLLER
```

**Common log patterns:**

| Log Message | Meaning |
| ------------- | --------- |
| `heartbeat sent successfully` | Console communication working |
| `CAPI registration failed: timeout` | Network issue reaching CrowdSec servers |
| `enrollment completed` | Console enrollment succeeded |
| `retrying enrollment (attempt 2/3)` | Temporary failure, automatic retry in progress |

## Related

- [CrowdSec Setup Guide](../guides/crowdsec-setup.md) — Beginner-friendly setup walkthrough
- [Web Application Firewall](./waf.md) — Complement CrowdSec with WAF protection
- [Access Control](./access-control.md) — Manual IP blocking and geo-restrictions
- [CrowdSec Troubleshooting](../troubleshooting/crowdsec.md) — Extended troubleshooting guide
- [Back to Features](../features.md)
