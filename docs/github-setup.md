---
title: GitHub Setup Guide
description: Configure GitHub Actions for automatic Docker builds and documentation deployment for Charon.
---

## GitHub Setup Guide

This guide will help you set up GitHub Actions for automatic Docker builds and documentation deployment.

---

## 📦 Step 1: Docker Image Publishing (Automatic!)

The Docker build workflow uses GitHub Container Registry (GHCR) to store your images. **No setup required!** GitHub automatically provides authentication tokens for GHCR.

### How It Works

GitHub Actions automatically uses the built-in secret token to authenticate with GHCR. We recommend creating a `GITHUB_TOKEN` secret (preferred); workflows currently still work with `CHARON_TOKEN` for backward compatibility.

- ✅ Push images to `ghcr.io/wikid82/charon`
- ✅ Link images to your repository
- ✅ Publish images for free (public repositories)

**Nothing to configure!** Just push code and images will be built automatically.

### Make Your Images Public (Optional)

By default, container images are private. To make them public:

1. **Go to your repository** → <https://github.com/Wikid82/charon>
2. **Look for "Packages"** on the right sidebar (after first build)
3. **Click your package name**
4. **Click "Package settings"** (right side)
5. **Scroll down to "Danger Zone"**
6. **Click "Change visibility"** → Select **"Public"**

**Why make it public?** Anyone can pull your Docker images without authentication!

---

## 📚 Step 2: Enable GitHub Pages (For Documentation)

Your documentation will be published to GitHub Pages (not the wiki). Pages is better for auto-deployment and looks more professional!

### Enable Pages

1. **Go to your repository** → <https://github.com/Wikid82/charon>
2. **Click "Settings"** (top menu)
3. **Click "Pages"** (left sidebar under "Code and automation")
4. **Under "Build and deployment":**
   - **Source**: Select **"GitHub Actions"** (not "Deploy from a branch")
5. That's it! No other settings needed.

Once enabled, your docs will be live at:

```
https://wikid82.github.io/charon/
```

**Note:** The first deployment takes 2-3 minutes. Check the Actions tab to see progress!

---

## � Step 3: Configure GitHub Secrets (For E2E Tests)

E2E tests require an emergency token to be configured in GitHub Secrets. This token allows tests to bypass security modules during teardown.

### Why This Is Needed

The emergency token is used by E2E tests to:
- Disable security modules (ACL, WAF, CrowdSec) after testing them
- Prevent cascading test failures due to leftover security state
- Ensure tests can always access the API regardless of security configuration

### Step-by-Step Configuration

1. **Generate emergency token:**

   **Linux/macOS:**
   ```bash
   openssl rand -hex 32
   ```

   **Windows PowerShell:**
   ```powershell
   [Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
   ```

   **Node.js (all platforms):**
   ```bash
   node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
   ```

   **Copy the output** (64 characters for hex, or appropriate length for base64)

2. **Navigate to repository secrets:**
   - Go to: `https://github.com/<your-username>/charon/settings/secrets/actions`
   - Or: Repository → Settings → Secrets and Variables → Actions

3. **Create new secret:**
   - Click **"New repository secret"**
   - **Name:** `CHARON_EMERGENCY_TOKEN`
   - **Value:** Paste the generated token
   - Click **"Add secret"**

4. **Verify secret is set:**
   - Secret should appear in the list
   - Value will be masked (cannot view after creation for security)

### Validation

The E2E workflow automatically validates the emergency token:

```yaml
- name: Validate Emergency Token Configuration
  run: |
    if [ -z "$CHARON_EMERGENCY_TOKEN" ]; then
      echo "::error::CHARON_EMERGENCY_TOKEN not configured"
      exit 1
    fi
```

If the secret is missing or invalid, the workflow will fail with a clear error message.

### Token Rotation

**Recommended schedule:** Rotate quarterly (every 3 months)

**Rotation steps:**

1. Generate new token (same method as above)
2. Update GitHub Secret:
   - Settings → Secrets → Actions
   - Click on `CHARON_EMERGENCY_TOKEN`
   - Click "Update secret"
   - Paste new value
   - Save
3. Update local `.env` file (for local testing)
4. Re-run E2E tests to verify

### Security Best Practices

✅ **DO:**
- Use cryptographically secure generation methods
- Rotate quarterly or after security events
- Store separately for local dev (`.env`) and CI/CD (GitHub Secrets)

❌ **DON'T:**
- Share tokens via email or chat
- Commit tokens to repository (even in example files)
- Reuse tokens across different environments
- Use placeholder or weak values

### Troubleshooting

**Error: "CHARON_EMERGENCY_TOKEN not set"**
- Check secret name is exactly `CHARON_EMERGENCY_TOKEN` (case-sensitive)
- Verify secret is repository-level, not environment-level
- Re-run workflow after adding secret

**Error: "Token too short"**
- Hex method must generate exactly 64 characters
- Verify you copied the entire token value
- Regenerate if needed

📖 **More Info:** See [E2E Test Troubleshooting Guide](troubleshooting/e2e-tests.md)

---

## �🚀 How the Workflows Work

### Docker Build Workflow (`.github/workflows/docker-build.yml`)

**Prerequisites:**

- go 1.25.7+ (automatically managed via `GOTOOLCHAIN: auto` in CI)
- Node.js 20+ for frontend builds

**Triggers when:**

- ✅ You push to `main` branch → Creates `latest` tag
- ✅ You push to `development` branch → Creates `dev` tag
- ✅ You create a version tag like `v1.0.0` → Creates version tags
- ✅ You manually trigger it from GitHub UI

**What it does:**

1. Builds the frontend
2. Builds a Docker image for multiple platforms (AMD64, ARM64)
3. Pushes to Docker Hub with appropriate tags
4. Tests the image by starting it and checking the health endpoint
5. Shows you a summary of what was built

**Tags created:**

- `latest` - Always the newest stable version (from `main`)
- `dev` - The development version (from `development`)
- `1.0.0`, `1.0`, `1` - Version numbers (from git tags)
- `sha-abc1234` - Specific commit versions

**Where images are stored:**

- `ghcr.io/wikid82/charon:latest`
- `ghcr.io/wikid82/charon:dev`
- `ghcr.io/wikid82/charon:1.0.0`

### Documentation Workflow (`.github/workflows/docs.yml`)

**Triggers when:**

- ✅ You push changes to `docs/` folder
- ✅ You update `README.md`
- ✅ You manually trigger it from GitHub UI

**What it does:**

1. Converts all markdown files to beautiful HTML pages
2. Creates a nice homepage with navigation
3. Adds dark theme styling (matches the app!)
4. Publishes to GitHub Pages
5. Shows you the published URL

---

## 🎯 Testing Your Setup

### Test Docker Build

1. Make a small change to any file
2. Commit and push to `development`:

   ```bash
   git add .
   git commit -m "test: trigger docker build"
   git push origin development
   ```

3. Go to **Actions** tab on GitHub
4. Watch the "Build and Push Docker Images" workflow run
5. Check **Packages** on your GitHub profile for the new `dev` tag!

### Test Docs Deployment

1. Make a small change to `README.md` or any doc file
2. Commit and push to `main`:

   ```bash
   git add .
   git commit -m "docs: update readme"
   git push origin main
   ```

3. Go to **Actions** tab on GitHub
4. Watch the "Deploy Documentation to GitHub Pages" workflow run
5. Visit your docs site (shown in the workflow summary)!

---

## 🏷️ Creating Version Releases

When you're ready to release a new version:

1. **Tag your release:**

   ```bash
   git tag -a v1.0.0 -m "Release version 1.0.0"
   git push origin v1.0.0
   ```

2. **The workflow automatically:**
   - Builds Docker image
   - Tags it as `1.0.0`, `1.0`, and `1`
   - Pushes to Docker Hub
   - Tests it works

3. **Users can pull it:**

   ```bash
   docker pull ghcr.io/wikid82/charon:1.0.0
   docker pull ghcr.io/wikid82/charon:latest
   ```

---

## 🐛 Troubleshooting

### Docker Build Fails

**Problem**: "Error: denied: requested access to the resource is denied"

- **Fix**: This shouldn't happen with `GITHUB_TOKEN` or `CHARON_TOKEN` - check workflow permissions
- **Verify**: Settings → Actions → General → Workflow permissions → "Read and write permissions" enabled

**Problem**: Can't pull the image

- **Fix**: Make the package public (see Step 1 above)
- **Or**: Authenticate with GitHub: `echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin` (or `CHARON_TOKEN` for backward compatibility)

### Docs Don't Deploy

**Problem**: "deployment not found"

- **Fix**: Make sure you selected "GitHub Actions" as the source in Pages settings
- **Not**: "Deploy from a branch"

**Problem**: Docs show 404 error

- **Fix**: Wait 2-3 minutes after deployment completes
- **Fix**: Check the workflow summary for the actual URL

### General Issues

**Check workflow logs:**

1. Go to **Actions** tab
2. Click the failed workflow
3. Click the failed job
4. Expand the step that failed
5. Read the error message

**Still stuck?**

- Open an issue: <https://github.com/Wikid82/charon/issues>
- We're here to help!

---

## 📋 Quick Reference

### Docker Commands

```bash
# Pull latest development version
docker pull ghcr.io/wikid82/charon:dev

# Pull stable version
docker pull ghcr.io/wikid82/charon:latest

# Pull specific version
docker pull ghcr.io/wikid82/charon:1.0.0

# Run the container
docker run -d -p 8080:8080 -v caddy_data:/app/data ghcr.io/wikid82/charon:latest
```

### Git Tag Commands

```bash
# Create a new version tag
git tag -a v1.2.3 -m "Release 1.2.3"

# Push the tag
git push origin v1.2.3

# List all tags
git tag -l

# Delete a tag (if you made a mistake)
git tag -d v1.2.3
git push origin :refs/tags/v1.2.3
```

### Trigger Manual Workflow

1. Go to **Actions** tab
2. Click the workflow name (left sidebar)
3. Click "Run workflow" button (right side)
4. Select branch
5. Click "Run workflow"

---

## ✅ Checklist

Before pushing to production, make sure:

- [ ] GitHub Pages is enabled with "GitHub Actions" source
- [ ] You've tested the Docker build workflow (automatic on push)
- [ ] You've tested the docs deployment workflow
- [ ] Container package is set to "Public" visibility (optional, for easier pulls)
- [ ] Documentation looks good on the published site
- [ ] Docker image runs correctly
- [ ] You've created your first version tag

---

## 🎉 You're Done

Your CI/CD pipeline is now fully automated! Every time you:

- Push to `main` → New `latest` Docker image + updated docs
- Push to `development` → New `dev` Docker image for testing
- Create a tag → New versioned Docker image

**No manual building needed!** 🚀

<p align="center">
   <em>Questions? Check the <a href="https://docs.github.com/en/actions">GitHub Actions docs</a> or <a href="https://github.com/Wikid82/charon/issues">open an issue</a>!</em>
</p>
