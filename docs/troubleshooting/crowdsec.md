# CrowdSec Troubleshooting

Keep Cerberus terminology and the Configuration Packages flow in mind while debugging Hub presets.

## Quick checks

- Cerberus is enabled and you are signed in with admin scope.
- `cscli` is available (preferred path); HTTPS CrowdSec Hub endpoints only.
  - Docker images (v1.7.4+): cscli is pre-installed.
  - Bare-metal deployments: install cscli for Hub preset sync or use HTTP fallback with HUB_BASE_URL.
- HUB_BASE_URL points to a JSON hub endpoint (default: <https://hub-data.crowdsec.net/api/index.json>). Redirects to HTML will be rejected.
- Proxy env is set when required: HTTP(S)_PROXY and NO_PROXY are respected by the hub client.
- For slow or proxied networks, increase HUB_PULL_TIMEOUT_SECONDS (default 25) and HUB_APPLY_TIMEOUT_SECONDS (default 45) to avoid premature timeouts.
- Preset workflow: pull from Hub using cache keys/ETags → preview changes → apply with automatic backup and reload flag.
- Preset pull/apply requires either cscli or cached presets.
- Offline/curated presets remain available at all times.

## Common issues

- Hub unreachable (503): retry once, then Charon falls back to cached Hub data if available; otherwise stay on curated/offline presets until connectivity returns.
- Hub returns HTML/redirect: set HUB_BASE_URL to the JSON endpoint above or install cscli so the index is fetched locally.
- Bad preset slug (400): the slug must match Hub naming; correct the slug before retrying.
- Apply failed: review the apply response and restore from the backup that was taken automatically, then retry after fixing the underlying issue.
- Apply not supported (501): use curated/offline presets; Hub apply will be re-enabled when supported in your environment.

## Tips

- Keep the CrowdSec Hub reachable over HTTPS; HTTP is blocked.
- If you switch to offline mode, clear pending Hub pulls before retrying so cache keys/ETags refresh cleanly.
- After restoring from a backup, re-run preview before applying again to verify changes.

## Console Enrollment

### "missing login field" or CAPI errors

Charon automatically attempts to register your instance with CrowdSec's Central API (CAPI) before enrolling. Ensure your server has internet access to `api.crowdsec.net`.

### Configuration File

Charon uses the configuration located in `data/crowdsec/config.yaml`. Ensure this file exists and is readable if you are manually modifying it.
