# CrowdSec Troubleshooting

Keep Cerberus terminology and the Configuration Packages flow in mind while debugging Hub presets.

## Quick checks
- Cerberus is enabled and you are signed in with admin scope.
- `cscli` is available (preferred path); HTTPS CrowdSec Hub endpoints only.
- Preset workflow: pull from Hub using cache keys/ETags → preview changes → apply with automatic backup and reload flag.
- Offline/curated presets remain available at all times.

## Common issues
- Hub unreachable (503): retry once, then Charon falls back to cached Hub data if available; otherwise stay on curated/offline presets until connectivity returns.
- Bad preset slug (400): the slug must match Hub naming; correct the slug before retrying.
- Apply failed: review the apply response and restore from the backup that was taken automatically, then retry after fixing the underlying issue.
- Apply not supported (501): use curated/offline presets; Hub apply will be re-enabled when supported in your environment.

## Tips
- Keep the CrowdSec Hub reachable over HTTPS; HTTP is blocked.
- If you switch to offline mode, clear pending Hub pulls before retrying so cache keys/ETags refresh cleanly.
- After restoring from a backup, re-run preview before applying again to verify changes.
