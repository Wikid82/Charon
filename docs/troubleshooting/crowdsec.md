---
title: CrowdSec Troubleshooting
description: Troubleshooting guide for CrowdSec integration issues in Charon. LAPI initialization, console enrollment, and common problems.
---

## CrowdSec Troubleshooting

Keep Cerberus terminology and the Configuration Packages flow in mind while debugging Hub presets.

### Quick checks

- Cerberus is enabled and you are signed in with admin scope.
- `cscli` is available (preferred path); HTTPS CrowdSec Hub endpoints only.
  - Docker images (v1.7.4+): cscli is pre-installed.
  - Bare-metal deployments: install cscli for Hub preset sync or use HTTP fallback with HUB_BASE_URL.
- HUB_BASE_URL points to a JSON hub endpoint (default: <https://cdn-hub.crowdsec.net/crowdsecurity/master>, whose index is `.index.json`; the GitHub mirror is the fallback, and `HUB_MIRROR_BASE_URL`, if you set it, is tried only after the default). Redirects to HTML will be rejected.
  - A loopback hub mirror is no longer reachable from the hub client; RFC 1918 hosts were already blocked.
  - The hub address must be an `https` address on the official CrowdSec hub hosts. Custom `http`, local, or other mirror addresses are not supported.
- Proxy env is set when required: HTTP(S)_PROXY and NO_PROXY are respected by the hub client.
- For slow or proxied networks, increase HUB_PULL_TIMEOUT_SECONDS (default 25) and HUB_APPLY_TIMEOUT_SECONDS (default 120) to avoid premature timeouts.
- The config file editor hides CrowdSec's account and connection credential files; they remain on disk (see `docker exec charon ls -la /app/data/crowdsec/config`).
- Backups sit next to the CrowdSec data folder, named `crowdsec.backup.<timestamp>` and `crowdsec.filebackup.<timestamp>`. Acquisition settings are backed up as `acquis.yaml.backup.<timestamp>` right beside the acquisition file. For manual recovery, run `docker exec charon ls -la /app/data` to find them, stop CrowdSec, copy the one you want back, then start it again.
- Preset workflow: pull a hub preset to preview it, then apply it. Apply installs exactly the item you previewed, with an automatic backup and rollback. Applying a hub preset needs `cscli` (the Docker image includes it).
- Offline/curated presets remain available at all times.

## LAPI Initialization and Timing

### Understanding LAPI Startup

When you enable CrowdSec via the GUI toggle, the Local API (LAPI) needs time to initialize before it's ready to accept requests. This is normal behavior.

**Typical startup times:**

- **Initial start:** 5-10 seconds
- **First start after container restart:** 10-15 seconds
- **Maximum wait:** 30 seconds (with automatic retries)

**What happens during startup:**

1. CrowdSec process starts
2. Configuration is loaded
3. Database connections are established
4. Parsers and scenarios are loaded
5. LAPI becomes available on port 8085
6. Status changes from "Starting" to "Active"

### Expected User Experience

When you toggle CrowdSec ON in the Security dashboard:

1. **Loading overlay appears** — "Starting CrowdSec... This may take up to 30 seconds"
2. **Backend polls LAPI** — Checks every 500ms for up to 30 seconds
3. **Success toast displays** — One of two messages:
   - ✅ "CrowdSec started and LAPI is ready" — You can immediately enroll in Console
   - ⚠️ "CrowdSec started but LAPI is still initializing" — Wait before enrolling

### Starting CrowdSec While It Is Already Running

Starting CrowdSec when it is already running does nothing. Charon answers the start request with the status `already_running` (plus the process ID and whether the Local API is ready) and leaves the existing process alone. If Charon lost track of a running CrowdSec process (for example, the process-tracking file was removed), Status and Stop find it again and take it over, and Stop waits for it to exit before reporting success. Charon recognises the engine by its executable name and its configuration file, so other programs that merely mention that file, such as `cscli` or `tail`, are never taken over or stopped.

When stopping, Charon first asks CrowdSec to shut down (SIGTERM) and waits up to 5 seconds. If the process is still running after that, Charon forces it to end (SIGKILL). If the process still cannot be ended, or the request is cancelled while Charon is waiting, Stop reports an error and keeps the process-tracking file so you can try again.

### Verifying LAPI Status

**Check if LAPI is running:**

```bash
docker exec charon cscli lapi status
```

**Expected output when ready:**

```text
✓ You can successfully interact with Local API (LAPI)
```

**If LAPI is not ready yet:**

```text
ERROR: connection refused
```

**Check LAPI health endpoint directly:**

```bash
docker exec charon curl -s http://localhost:8085/health
```

**Expected response when healthy:**

```json
{"status":"up"}
```

### Troubleshooting LAPI Initialization

#### Problem: LAPI takes longer than 30 seconds

**Symptoms:**

- Warning message: "LAPI is still initializing"
- Console enrollment fails with "LAPI not available"

**Solution 1 - Wait and retry:**

```bash
# Wait 15 seconds, then check again
sleep 15
docker exec charon cscli lapi status
```

**Solution 2 - Check CrowdSec logs:**

```bash
docker logs charon | grep -i crowdsec | tail -20
```

Look for:

- ✅ "CrowdSec Local API listening" — LAPI started successfully
- ✅ "parsers loaded" — Configuration loaded
- ❌ "error" or "fatal" — Initialization problem

**Solution 3 - Turn CrowdSec off and on again:**

1. Go to Security dashboard
2. Toggle CrowdSec **OFF**
3. Wait 5 seconds
4. Toggle CrowdSec **ON**
5. Wait 15 seconds
6. Verify status shows "Active"

#### Problem: LAPI never becomes available

**Check if CrowdSec process is running:**

```bash
docker exec charon ps aux | grep crowdsec
```

**Expected output:**

```text
crowdsec  203  0.5  2.3  /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml
```

**If no process is running:**

1. Check config directory exists:

   ```bash
   docker exec charon ls -la /app/data/crowdsec/config
   ```

2. If directory is missing:

   ```bash
   docker compose restart
   ```

3. Check for port conflicts:

   ```bash
   docker exec charon netstat -tulpn | grep 8085
   ```

4. Remove deprecated environment variables from docker-compose.yml (see migration section below)

#### Problem: LAPI responds but enrollment fails

**Check LAPI can process requests:**

```bash
docker exec charon cscli machines list
```

**Expected output:**

```text
Name                     IP Address      Auth Type    Version
charon-local-machine     127.0.0.1      password     v1.x.x
```

**If command fails:**

- LAPI is running but database is not ready
- Wait 10 more seconds and retry
- Check logs for database errors

**If enrollment still fails:**

- Enrollment has automatic retry (3 attempts, 2 seconds apart)
- If all retries fail, toggle CrowdSec OFF/ON and try again
- See Console Enrollment section below for token troubleshooting

## Common issues

- Hub unreachable (503, `hub_unreachable`): retry once, then Charon falls back to cached Hub data if available; otherwise stay on curated/offline presets until connectivity returns. The same answer is given when a pull needs to download a preset that is not cached and the hub cannot be reached, and when an apply cannot download an item.
- Hub returns HTML/redirect: set HUB_BASE_URL to the JSON endpoint above or install cscli so the index is fetched locally.
- Bad preset slug (400): the slug must match Hub naming; correct the slug before retrying.
- **"Install missing hub items" says the hub could not be reached**: nothing was installed and nothing was changed. Check that the container has internet access and can resolve names (DNS), then click the button again. Items that were not reached in time can be retried with **Install remaining**.
- **"The archive keeps its configuration at the top level; CrowdSec reads it from the config/ folder. Export a configuration from this page, or put the files in a config/ folder inside the archive. Nothing was changed."**: do what the message says. Exporting from the CrowdSec page always gives you the right layout. See the [CrowdSec feature guide](../features/crowdsec.md).
- **"The archive's profiles.yaml names notifications that no file in the archive defines: ..."**: your imported alert profiles refer to notifications that are not in the archive. Add the missing notification files, or remove those names from the profile, then import again. Nothing was changed.
- **"The archive has no profiles.yaml, so the current one was kept, but it names notifications the archive has no files for: ..."**: Charon kept your current alert profile, but it needs notification files the archive does not have. Add those files to the archive, or include your own `profiles.yaml`. Nothing was changed.
- **"The archive has no profiles.yaml, so the current one was kept, but a notification file it needs defines a name the archive already defines."**: the kept profile needs a notification file that clashes with one in your archive. Remove or rename the duplicate in the archive, or include your own `profiles.yaml`, then import again. Nothing was changed.
- **"The current CrowdSec files could not be kept, so the import was stopped. See the server logs for details."**: Charon could not copy back the files an import must keep, so it put everything back as it was. Check that the data folder has free space and is writable, run `docker logs charon` for the reason, then try again.
- **"config.yaml was not found at the top level or in the config/ folder of the archive."**: the archive has no main CrowdSec settings file. Export a configuration from the CrowdSec page, or add `config.yaml` to the archive's `config/` folder, then import again. Nothing was changed.
- **"config.yaml is not valid YAML" or "invalid YAML" for a very large or deeply nested file**: Charon refuses YAML files over 2 MiB or ones that expand into an extreme number of entries (for example, heavy use of aliases), because reading them could slow CrowdSec down. Simplify the file or split it up, then try again.
- **"The CrowdSec configuration file could not be exported safely"** (HTTP 422, `export_config_unsafe`): the export hides credentials in `config.yaml` and `config.yaml.local`, and it refuses to continue when it cannot do that reliably: the file is larger than 1 MiB, is not valid YAML, or shares credential values through YAML anchors, aliases or merge keys. Nothing is exported, because an archive without the main configuration could not be imported again. Fix or simplify the file (the log names it), then export again. The credential hiding is a best-effort list of well-known setting names, so keep treating an export as private.
- **Private key files are missing after restoring an export on another host**: exports leave out private key files so they never travel inside a shared file. The export response lists what was left out (up to 50 files, in the `X-Charon-Omitted-Files` header); the Charon log records how many. Charon detects them by name (`*.key`, `*.p12`, `*.pfx`, or a file a notification names as `key_path`) or by a text-format (PEM or PGP) private key inside the file (whole files up to 1 MiB; only the first 64 KiB of larger ones); a binary key with any other name is not detected, so rename key files to end in `.key`. Copy those files onto the new host yourself. Importing on the same server keeps your current key files. See the [CrowdSec feature guide](../features/crowdsec.md).
- **An imported notification does not send, or the import card says files "still need values"**: exports blank passwords, tokens and webhook addresses in notification files, and only notifications that already exist on this system with the same type, name and target get their values back. Open each listed file in the config file editor and enter the real values, or import again after adding them. If a file used YAML anchors or aliases and came back blanked on the same system, your earlier values are in the `.backup.` copy Charon made next to your data folder before the import. See the [CrowdSec feature guide](../features/crowdsec.md).
- Apply failed: review the apply response and restore from the backup that was taken automatically, then retry after fixing the underlying issue. If the automatic restore failed too, Charon keeps that backup for manual recovery and puts a small `.keep` file next to it. A kept backup is protected for 7 days. After that, only the newest 3 kept backups keep their `.keep` file, and an older one that loses it becomes eligible for normal cleanup, which removes the oldest backups once more than the usual number are kept. To release a backup for cleanup sooner, delete its `.keep` file.
- Apply not supported (501): use curated/offline presets; Hub apply will be re-enabled when supported in your environment.
- **I changed the CrowdSec key in my compose file, but Charon still uses the old one**: Charon keeps the key it saved first, as long as CrowdSec still accepts it. Delete the saved file `/app/data/crowdsec/bouncer_key` (inside your data folder) and restart Charon. Charon then uses the key from your compose file if CrowdSec accepts it; otherwise it creates a new key. Details are in the [CrowdSec feature guide](../features/crowdsec.md#managing-the-crowdsec-bouncer-key).
- **Security Engine Offline**: If your dashboard says "Offline", it means CrowdSec LAPI is not running.
  - **Fix**: Ensure CrowdSec is **enabled via GUI toggle** in the Security dashboard. Do NOT use environment variables.
  - **Action**: Go to Security dashboard, toggle CrowdSec ON, wait 15 seconds, verify status shows "Active".
- **CrowdSec is on, but your sites are not using it, and the log shows a warning about a missing bouncer key**: CrowdSec only joins your proxy once a bouncer key exists. Charon creates one automatically the first time CrowdSec starts. Until then, the proxy leaves CrowdSec out and your other settings keep working. Check the log with `docker logs charon`. If the key never appears, see [Managing the CrowdSec bouncer key](../features/crowdsec.md#managing-the-crowdsec-bouncer-key).
- **Turning CrowdSec on or off did not update the proxy**: the Security page still shows CrowdSec's real state, and the log has a warning. Switch CrowdSec off and on again to retry.

## Ban and unban messages

These appear when you ban or unban an address on the Live Decisions page. Nothing is changed when one of them shows.

- **"Enter a valid IPv4 or IPv6 address, or a range such as 192.0.2.0/24."**: the address is not valid. Check for typos, spaces or a leading dash.
- **"Choose a duration between 1 minute and 365 days, for example 4h, 30m or 1d."**: use `m` (minutes), `h` (hours) or `d` (days). Weeks (`1w`) and capital letters (`4H`) are not accepted.
- **"The reason can be at most 200 characters."**: shorten the reason.
- **"No decision with exactly this address or range."**: nothing is blocked under that exact address or range, perhaps because it already expired or was unbanned. Unbanning removes only an exact match: a range does not clear single addresses banned inside it, and an address does not clear a range that covers it. Refresh the list and use the value exactly as shown.
- **A "request too large" error** (HTTP 413, `request_too_large`): CrowdSec requests are limited to 64 KiB. Shorten the text you sent.
- **"CrowdSec did not answer in time. Try again."** (also shown as "CrowdSec did not answer in time" on other CrowdSec pages): CrowdSec's local service is stopped or too slow. Check that it is running (see [Verifying LAPI Status](#verifying-lapi-status)), wait a moment, then try again.

## List notes

The Live Decisions, dashboard and alerts lists show a short note instead of an error when CrowdSec could not supply the list. The list is empty or partial, and it refreshes on its own, so no action is needed unless the note stays.

- **"The decision service took too long to answer, so this list may be incomplete."** (`cscli_timeout`): CrowdSec's local service is slow or stopped. See [Verifying LAPI Status](#verifying-lapi-status).
- **"The decision service returned more data than can be shown, so this list may be incomplete."** (`cscli_output_too_large`): there are too many entries to show at once. Pick a shorter time range.
- **"The decision service is not available right now, so this list may be incomplete."** (`cscli_unavailable`): Charon could not run `cscli`. Restart the container.

## Acquisition settings messages

These appear when Charon cannot read or save your log sources settings file (`acquis.yaml`, in the `crowdsec/config` folder inside your data folder). Nothing is changed when one of them shows.

- **Error code `acquisition_config_unreadable`**: Charon cannot open the file. The message asks you to check the file's owner and permissions.
- **Error code `acquisition_config_not_writable`**: Charon cannot save changes to the file, usually because the file or its folder is read-only. The message asks you to check the file's owner and permissions.

**How to fix it:**

1. Look at the file and its folder. Run `docker exec charon ls -la /app/data/crowdsec/config` and check the line for `acquis.yaml`. The first column shows who may read and write it (for example `-rw-r--r--`), and the owner name is shown further along.
2. Make sure the folder is writable too. Run `docker exec charon ls -ld /app/data/crowdsec/config`. A folder that only lets its owner read it cannot have files saved in it.
3. Fix the owner or permissions on your computer, then restart Charon. `chown` changes the owner of a file or folder, and `chmod` changes who may read or write it. Ask someone who knows your server if you are not sure which owner to use.
4. If you mounted your data folder as read-only (a `:ro` at the end of its line in your compose file), Charon cannot save anything. Remove the `:ro` from that line, restart Charon, and try again.

## Hub preset messages

Applying a hub preset shows one of these messages when it cannot finish or needs your attention. When an apply fails, your CrowdSec settings are put back as they were.

- **"The preview is out of date. Pull the preset again, then apply it."**: the preview is too old or was replaced. Click **Pull** again, then **Apply**.
- **"The hub is still updating, try again later."** (HTTP 409, `preset_hub_updating`): the hub is still publishing a newer version of this item. Charon checks every downloaded item against the hub's index before it keeps a copy, and refuses one that does not match, so an incomplete update is not used. Wait a while, pull the preset again, then apply it.
- **"This item was edited on this server and was left unchanged. Reset it before applying the preset."**: you (or a script) changed this item's file on this server, and Charon will not overwrite your edit. The message lists the edited items ("Edited items: ..."). A plain install or upgrade does nothing for an edited item, so reset it with `cscli <type> install <name> --force` (Docker users run it as `docker exec charon cscli ...`), for example `docker exec charon cscli parsers install crowdsecurity/apache2-logs --force`. If a collection reports an edited item inside it, run `cscli collections install <collection> --force`, which also restores that item. This replaces your edit with the original hub version, so your local change is lost. Then reload CrowdSec, pull the preset again and apply it.
- **"The preset install took too long and was rolled back. Try again."**: the install did not finish within 120 seconds (typical installs take 1-5 seconds; the slowest common collection takes about a minute on a first download). Try again, or raise `HUB_APPLY_TIMEOUT_SECONDS` on a slow connection (the web interface stops waiting after 130 seconds, so values above about 125 only help when you run the install yourself with `cscli`). Very large collections (hundreds of items, such as `sigmahq/windows_proc_creation`) can take several minutes because items are downloaded one by one; install those directly with `docker exec charon cscli collections install <name>`.
- **"CrowdSec could not reach the hub. Check the network connection and try again."** (HTTP 503, `hub_unreachable`): the server has no route to the CrowdSec hub, when pulling a preset that was never downloaded as well as when applying one. Nothing was installed and your settings are unchanged. Check the container's internet and DNS access, then try again.
- **"This name exists in several hub sections. Choose which item to use."** (HTTP 409, `preset_type_ambiguous`): pick the item type in the list that appears, then pull again.
- **"cscli unavailable or not responding"**: Charon could not run `cscli`. The Docker image includes it; restart the container and try again.
- **"cscli unavailable or not responding" offline**: an already-downloaded preview can be applied if its items are already on the server; otherwise the apply fails and is rolled back. Pulling a preset that was never downloaded needs internet access.
- **A note that some dependencies were edited on this server**: the preset was applied, and those edited dependencies were left unchanged.

### Large preset installs

- **"CrowdSec is busy installing a preset; try again when it finishes."** (HTTP 409, `crowdsec_busy`): a preset install is running in the background, and other CrowdSec changes are paused until it ends. Wait for the progress row to finish, or click **Cancel** on it, then try again.
- **"CrowdSec is finishing another change. Try again in a moment."** (HTTP 409, `crowdsec_busy`, no install running): you tried to start a preset install while another CrowdSec change, such as turning CrowdSec on or off, was still finishing. Wait a moment and try again.
- **The install shows as interrupted**: Charon restarted while the install was running. Your previous settings were restored automatically. Start the install again.
- **The install shows as cancelled**: you cancelled it. Your previous settings were put back. Nothing else is needed.
- **The install shows as timed out**: it did not finish within 30 minutes and was rolled back. Check the container's internet connection, then try again. For very large collections you can also run `docker exec charon cscli collections install <name>`.
- **"The preview is out of date"** (`preset_cache_stale`) or **"CrowdSec could not reach the hub"** (`hub_unreachable`): see the messages above, then pull the preset again.

## Switch, setting and reset messages

These appear when you turn CrowdSec on or off, change its setting, or run an emergency reset. Nothing is changed when one of them shows, unless the entry says otherwise.

- **"CrowdSec was turned off while it was starting."** (error code `crowdsec_state_changed`, HTTP 409): you turned CrowdSec off, or ran an emergency reset, while it was still starting. CrowdSec stays off and the setting stays off. Check the **CrowdSec Protection** switch on the Security page, then turn it on again if you still want it.
- **Error code `crowdsec_setting_use_switch`** (HTTP 400): a settings change tried to set CrowdSec's internal mode value directly. Turn CrowdSec on or off with the **CrowdSec Protection** switch on the Security page instead. This only affects scripts or other tools that call the settings API.
- **Error code `invalid_crowdsec_setting`** (HTTP 400): the CrowdSec setting (`security.crowdsec.enabled`) must be `true` or `false`. Values such as `yes` or `1` are refused. Use `true` to turn CrowdSec on or `false` to turn it off.
- **"CrowdSec is not available on this server."** (error code `crowdsec_unavailable`, HTTP 503): the part of Charon that starts and stops CrowdSec is not running, so a switch cannot change CrowdSec. Restart the container and try again. If it keeps showing, run `docker logs charon` and look for the reason. An emergency reset still turns off the other security modules in this case, but CrowdSec may keep running until you restart Charon.
- **"Charon manages this file. Change the allow list on the Whitelist tab."** (error code `file_managed_by_charon`, HTTP 400): you tried to save `charon-whitelist.yaml` in the config file editor. The file is not changed. Make your change on the **Whitelist** tab instead, because Charon rewrites this file from that list every time CrowdSec starts. Hand edits to the file are replaced the same way at the next start.
- **The log says CrowdSec is running while the security suite is off**: this is information, not an error. CrowdSec is running, but your proxy leaves it out until the security suite is turned on again. Turn the security suite on from the Security page if you want CrowdSec to protect your sites.

## CrowdSec Not Starting After Container Restart

### Problem: Toggle shows ON but CrowdSec is not running

**Symptoms:**

- Container restarted (reboot, Docker restart, etc.)
- Security dashboard toggle shows "ON"
- Status badge shows "Not Running" or "Offline"
- Manually toggling OFF then ON fixes it

**Root Cause:**

The reconciliation function couldn't determine if CrowdSec should auto-start. This happens when:

1. **SecurityConfig table is missing/corrupted** (database issue)
2. **Settings table and SecurityConfig are out of sync** (partial update)
3. **Reconciliation logs show silent exit** (no "starting based on" message)

### Diagnosis: Check Reconciliation Logs

**View container startup logs:**

```bash
docker logs charon | grep -i "crowdsec reconciliation"
```

**Expected output when working correctly:**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"..."}
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'","time":"..."}
{"level":"info","msg":"CrowdSec Local API listening on 127.0.0.1:8085","time":"..."}
```

**Problematic output (silent exit - BUG):**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"..."}
[NO FURTHER LOGS - Function exited without starting CrowdSec]
```

This indicates reconciliation found conflicting state between Settings and SecurityConfig tables.

### Solution 1: Verify Database State

**Check Settings table:**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT key, value FROM settings WHERE key = 'security.crowdsec.enabled';"
```

**Expected output:**

```text
security.crowdsec.enabled|true
```

**Check SecurityConfig table:**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT uuid, crowdsec_mode, enabled FROM security_configs WHERE uuid = 'default';"
```

**Expected output:**

```text
default|local|1
```

**Mismatch scenarios:**

| Settings | SecurityConfig | Behavior | Fix Needed |
| ---------- | ---------------- | ---------- | ------------ |
| `true` | `local` | ✅ Auto-starts | None |
| `true` | `disabled` | ❌ Does NOT start | Run Solution 2 |
| `true` | (missing) | ⚠️ Should auto-create | Run Solution 3 |
| `false` | `local` | ⚠️ Conflicting state | Run Solution 2 |
| `false` | `disabled` | ✅ Correctly skipped | None (expected) |

### Solution 2: Manually Sync SecurityConfig to Settings

**If you want CrowdSec enabled (Settings = true, SecurityConfig = disabled):**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE security_configs SET crowdsec_mode = 'local', enabled = 1 WHERE uuid = 'default';"

docker restart charon
```

**If you want CrowdSec disabled (Settings = false, SecurityConfig = local):**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE security_configs SET crowdsec_mode = 'disabled', enabled = 0 WHERE uuid = 'default';"

# Also update Settings for consistency
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE settings SET value = 'false' WHERE key = 'security.crowdsec.enabled';"

docker restart charon
```

### Solution 3: Force Recreation of SecurityConfig

**If SecurityConfig table is missing (record not found):**

```bash
# Delete SecurityConfig (if partial record exists)
docker exec charon sqlite3 /app/data/charon.db \
  "DELETE FROM security_configs WHERE uuid = 'default';"

# Restart container - reconciliation will auto-create matching Settings state
docker restart charon

# Wait 15 seconds for startup
sleep 15

# Verify CrowdSec started
docker exec charon cscli lapi status
```

**Expected behavior:**

- Reconciliation detects missing SecurityConfig
- Checks Settings table for user preference
- Creates SecurityConfig with matching state
- Starts CrowdSec if Settings = true

**Check logs to confirm:**

```bash
docker logs charon | grep "default SecurityConfig created"
```

Expected:

```json
{"level":"info","msg":"CrowdSec reconciliation: default SecurityConfig created from Settings preference","crowdsec_mode":"local","enabled":true,"source":"settings_table"}
```

### Solution 4: Use GUI Toggle (Safest)

**The GUI toggle synchronizes both tables atomically:**

1. Go to **Security** dashboard
2. Toggle CrowdSec **OFF** (if it shows ON)
3. Wait 5 seconds
4. Toggle CrowdSec **ON**
5. Wait 15 seconds for LAPI to initialize
6. Verify status shows "Active"

**Why this works:**

- Toggle updates Settings table
- Toggle updates SecurityConfig table
- Start handler ensures both tables match
- Future restarts use reconciliation correctly

### Solution 5: Manual Reset (Nuclear Option)

**If all else fails, reset both tables:**

```bash
# Stop CrowdSec if running
docker exec charon pkill crowdsec || true

# Reset both tables
docker exec charon sqlite3 /app/data/charon.db <<EOF
UPDATE settings SET value = 'false' WHERE key = 'security.crowdsec.enabled';
DELETE FROM security_configs WHERE uuid = 'default';
EOF

# Restart container
docker restart charon

# Re-enable via GUI
# Go to Security dashboard and toggle CrowdSec ON
```

### Prevention: Verify After Manual Database Changes

**If you manually edit the database:**

```bash
# Always verify both tables match
docker exec charon sqlite3 /app/data/charon.db <<EOF
SELECT 'Settings:' as table_name, value as state
FROM settings WHERE key = 'security.crowdsec.enabled'
UNION ALL
SELECT 'SecurityConfig:', crowdsec_mode
FROM security_configs WHERE uuid = 'default';
EOF
```

**Expected output (both enabled):**

```text
Settings:|true
SecurityConfig:|local
```

**Expected output (both disabled):**

```text
Settings:|false
SecurityConfig:|disabled
```

### When to Contact Support

If after following all solutions:

- ❌ Reconciliation logs still show silent exit
- ❌ Both tables show correct state but CrowdSec doesn't start
- ❌ Manual `cscli lapi status` fails even after toggle

**Gather diagnostic info:**

```bash
# Collect logs
docker logs charon > charon-logs.txt 2>&1

# Collect database state
docker exec charon sqlite3 /app/data/charon.db ".dump security_configs" > db-state.sql
docker exec charon sqlite3 /app/data/charon.db ".dump settings" >> db-state.sql

# Collect process state
docker exec charon ps aux > process-state.txt
```

**Report issue:** <https://github.com/Wikid82/charon/issues>

Include:

- Output of all diagnostic commands above
- Steps you tried from this guide
- Container restart logs showing reconciliation behavior

## Tips

- Keep the CrowdSec Hub reachable over HTTPS; HTTP is blocked.
- If you switch to offline mode, clear pending Hub pulls before retrying so cache keys/ETags refresh cleanly.
- After restoring from a backup, re-run preview before applying again to verify changes.
- **Always use the GUI toggle** for enabling/disabling CrowdSec—it ensures Settings and SecurityConfig stay synchronized.
- **Check reconciliation logs** after container restart to verify auto-start behavior.

## Database Migrations After Upgrade

### Problem: CrowdSec not starting after upgrading Charon

**Symptoms:**

- CrowdSec toggle appears enabled but status shows "Not Running"
- CrowdSec console shows "Starting..." indefinitely
- Container logs show: `WARN CrowdSec reconciliation: security tables missing`
- Console enrollment fails immediately

**Root Cause:**

Upgrading from an older version with a **persistent database** may be missing the new security tables introduced in version 2.0. The database schema needs to be migrated.

**Solution: Run Database Migration**

1. **Execute the migration command:**

   ```bash
   docker exec charon /app/charon migrate
   ```

   **Expected output:**

   ```json
   {"level":"info","msg":"Running database migrations for security tables...","time":"..."}
   {"level":"info","msg":"Migration completed successfully","time":"..."}
   ```

2. **Verify tables were created:**

   ```bash
   docker exec charon sqlite3 /app/data/charon.db ".tables"
   ```

   **Expected tables include:**
   - `security_configs`
   - `security_decisions`
   - `security_audits`
   - `security_rule_sets`
   - `crowdsec_preset_events`
   - `crowdsec_console_enrollments`

3. **Restart container to apply changes:**

   ```bash
   docker restart charon
   ```

4. **Verify CrowdSec starts automatically:**

   If you had CrowdSec enabled before the upgrade:

   ```bash
   # Wait 15 seconds after restart, then check
   docker exec charon cscli lapi status
   ```

   **Expected output:**

   ```text
   ✓ You can successfully interact with Local API (LAPI)
   ```

5. **If CrowdSec doesn't auto-start:**

   Enable it manually via the GUI:
   - Go to **Security** dashboard
   - Toggle CrowdSec **ON**
   - Wait 15 seconds
   - Verify status shows "Active"

**Why This Happens:**

Charon version 2.0 moved CrowdSec configuration from environment variables to the database (see [Migration Guide](../migration-guide.md)). Persistent databases from older versions need the new security tables added via migration.

**Prevention:**

Future upgrades will run migrations automatically on startup. For now, manual migration is required for existing installations.

**Related Documentation:**

- [Getting Started - Database Migrations](../getting-started.md#step-15-database-migrations-if-upgrading)
- [Migration Guide - CrowdSec Control](../migration-guide.md)

---

## Console Enrollment

### Prerequisites

Before attempting Console enrollment, ensure:

✅ **CrowdSec is enabled** — Toggle must be ON in Security dashboard
✅ **LAPI is running** — Check with: `docker exec charon cscli lapi status`
✅ **Feature flag enabled** — `feature.crowdsec.console_enrollment` must be ON
✅ **Valid token** — Obtain from crowdsec.net

### "missing login field" or CAPI errors

Charon automatically attempts to register your instance with CrowdSec's Central API (CAPI) before enrolling. Ensure your server has internet access to `api.crowdsec.net`.

### Enrollment shows "enrolled" but not on crowdsec.net

**Root cause:** LAPI was not running when enrollment was attempted.

Charon now checks LAPI availability before enrollment and retries automatically (3 attempts with 2-second delays), but in rare cases enrollment may still fail if LAPI is initializing.

**Solution:**

1. Verify LAPI status:

   ```bash
   docker exec charon cscli lapi status
   ```

   **Expected output when ready:**

   ```text
   ✓ You can successfully interact with Local API (LAPI)
   ```

   **If LAPI is not running:**

   ```text
   ERROR: cannot contact local API
   ```

2. If LAPI is not running:
   - Go to Security dashboard
   - Toggle CrowdSec **OFF**
   - Wait 5 seconds
   - Toggle CrowdSec **ON**
   - **Wait 15 seconds** (important: LAPI needs time to initialize)
   - Re-check LAPI status

3. Verify LAPI health endpoint:

   ```bash
   docker exec charon curl -s http://localhost:8085/health
   ```

   **Expected response:**

   ```json
   {"status":"up"}
   ```

4. Re-submit enrollment token:
   - Go to **Cerberus → CrowdSec**
   - Click **Enroll with CrowdSec Console**
   - Paste the same enrollment token (tokens are reusable)
   - Click **Submit**
   - Wait 30-60 seconds for confirmation

5. Verify enrollment on crowdsec.net:
   - Log in to your CrowdSec Console account
   - Navigate to **Instances**
   - Your Charon instance should appear in the list

**Understanding the automatic retry:**

Charon automatically retries LAPI checks during enrollment:

- **Attempt 1:** Immediate check
- **Attempt 2:** After 2 seconds (if LAPI not ready)
- **Attempt 3:** After 4 seconds (if still not ready)
- **Total:** 3 attempts over 6 seconds

This handles most cases where LAPI is still initializing. If all 3 attempts fail, follow the solution above.

### CrowdSec won't start via GUI toggle

**Solution:**

1. Check container logs:

   ```bash
   docker logs charon | grep -i crowdsec
   ```

   Look for:
   - ✅ "Starting CrowdSec Local API"
   - ✅ "CrowdSec Local API listening on 127.0.0.1:8085"
   - ❌ "failed to start" or "error loading config"

2. Verify config directory:

   ```bash
   docker exec charon ls -la /app/data/crowdsec/config
   ```

   Expected files:
   - `config.yaml` — Main configuration
   - `local_api_credentials.yaml` — LAPI authentication
   - `acquis.yaml` — Log sources

3. Check for common startup errors:

   **Error: "config.yaml not found"**

   ```bash
   # Restart container to regenerate config
   docker compose restart
   ```

   **Error: "port 8085 already in use"**

   ```bash
   # Check for conflicting services
   docker exec charon netstat -tulpn | grep 8085
   # Stop conflicting service or change CrowdSec LAPI port
   ```

   **Error: "permission denied"**

   ```bash
   # Fix ownership (run on host)
   sudo chown -R 1000:1000 ./data/crowdsec
   docker compose restart
   ```

4. Remove any deprecated environment variables from docker-compose.yml:

   ```yaml
   # REMOVE THESE:
   - CHARON_SECURITY_CROWDSEC_MODE=local
   - CERBERUS_SECURITY_CROWDSEC_MODE=local
   - CPM_SECURITY_CROWDSEC_MODE=local
   ```

5. Restart and try GUI toggle again:

   ```bash
   docker compose restart
   # Wait 30 seconds for container to fully start
   # Then toggle CrowdSec ON in GUI
   ```

6. Verify CrowdSec is running:

   ```bash
   # Check process
   docker exec charon ps aux | grep crowdsec

   # Check LAPI health
   docker exec charon cscli lapi status

   # Check LAPI endpoint
   docker exec charon curl -s http://localhost:8085/health
   ```

### Environment Variable Migration

🚨 **DEPRECATED:** The `CHARON_SECURITY_CROWDSEC_MODE` environment variable is no longer used.

If you have this in your docker-compose.yml, remove it and use the GUI toggle instead. See [Migration Guide](../migration-guide.md) for step-by-step instructions.

### Configuration File

Charon uses the configuration located in `data/crowdsec/config.yaml`. Ensure this file exists and is readable if you are manually modifying it.
