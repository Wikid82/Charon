# Troubleshooting: React Production Build Errors

## "Cannot set properties of undefined" Error

If you encounter this error when running Charon in production (typically appearing as "Cannot set properties of undefined (setting 'root')" in the browser console), this is usually caused by stale browser cache or outdated Docker images.

### Quick Fixes

#### 1. Hard Refresh Browser

Clear the browser cache and force a full reload:

- **Chrome/Edge:** `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
- **Firefox:** `Ctrl+F5` (Windows/Linux) or `Cmd+Shift+R` (Mac)
- **Safari:** `Cmd+Option+R` (Mac)

#### 2. Clear Browser Cache

Open Browser DevTools and clear all site data:

1. Open DevTools (F12 or right-click → Inspect)
2. Navigate to **Application** tab (Chrome/Edge) or **Storage** tab (Firefox)
3. Click **Clear Site Data** or **Clear All**
4. Reload the page

#### 3. Rebuild Docker Image

If the error persists after clearing browser cache, your Docker image may be outdated:

```bash
# Stop and remove the current container
docker compose -f .docker/compose/docker-compose.yml down

# Rebuild with no cache
docker compose -f .docker/compose/docker-compose.yml up -d --build --no-cache
```

#### 4. Verify Docker Image Tag

Check that you're running the latest version:

```bash
docker images charon/app --format "{{.Tag}}" | head -1
```

Compare with the latest release at: <https://github.com/Wikid82/charon/releases>

### Root Cause

This error typically occurs when:

1. **Browser cached old JavaScript files** that are incompatible with the new HTML template
2. **Docker image wasn't rebuilt** after updating dependencies
3. **CDN or proxy cache** is serving stale assets

React 19.2.3 and lucide-react@0.562.0 are fully compatible and tested. The issue is almost always environment-related, not a code bug.

### Still Having Issues?

If the error persists after trying all fixes above, please report the issue with:

- **Browser console screenshot** (DevTools → Console tab, screenshot the full error)
- **Browser name and version** (e.g., Chrome 120.0.6099.109)
- **Docker image tag** (from `docker images` command)
- **Any browser extensions enabled** (especially ad blockers or privacy tools)
- **Steps to reproduce** (what page you visited, what you clicked)

Open an issue at: <https://github.com/Wikid82/charon/issues>

### Prevention

To avoid this issue in the future:

1. **Always rebuild Docker images** when upgrading Charon:

   ```bash
   docker compose pull
   docker compose up -d --build
   ```

2. **Clear browser cache** after major updates

3. **Use versioned Docker tags** instead of `:latest` to avoid unexpected updates:

   ```yaml
   image: ghcr.io/wikid82/charon:v0.4.0
   ```

### Technical Background

React's production build uses optimized bundle splitting and code minification. When the browser caches old JavaScript chunks but receives a new HTML template, the chunks may reference objects that don't exist in the new bundle structure, causing "Cannot set properties of undefined" errors.

**Verified Compatible Versions:**

- React: 19.2.3
- React DOM: 19.2.3
- lucide-react: 0.562.0
- Vite: 7.3.0

All 1403 unit tests pass, and production builds succeed without errors. See [diagnostic report](../implementation/react-19-lucide-error-DIAGNOSTIC-REPORT.md) for full test results.
