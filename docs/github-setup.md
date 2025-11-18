# 🔧 GitHub Setup Guide

This guide will help you set up GitHub Actions for automatic Docker builds and documentation deployment.

---

## 📦 Step 1: Set Up GitHub Container Registry Token

The Docker build workflow uses GitHub Container Registry (GHCR) to store your images.

### Add Your GHCR Token:

1. **Create a Personal Access Token (PAT):**
   - Go to https://github.com/settings/tokens
   - Click **"Generate new token"** → **"Generate new token (classic)"**
   - Give it a name: `CPMP GHCR Token`
   - Select scopes:
     - ✅ `write:packages` (upload container images)
     - ✅ `read:packages` (download container images)
     - ✅ `delete:packages` (optional - delete old images)
   - Click **"Generate token"**
   - **Copy the token** - you won't see it again!

2. **Add Secret to Repository:**
   - Go to your repository → https://github.com/Wikid82/CaddyProxyManagerPlus
   - Click **"Settings"** → **"Secrets and variables"** → **"Actions"**
   - Click **"New repository secret"**
   - Name: `CPMP_GHCR_TOKEN`
   - Value: Paste the token you copied
   - Click **"Add secret"**

**Where images go:**
- ✅ `ghcr.io/wikid82/caddyproxymanagerplus`
- ✅ Images are linked to your repository
- ✅ Free for public repositories

### Make Your Images Public (Optional):

By default, container images are private. To make them public:

1. **Go to your repository** → https://github.com/Wikid82/CaddyProxyManagerPlus
2. **Look for "Packages"** on the right sidebar (after first build)
3. **Click your package name**
4. **Click "Package settings"** (right side)
5. **Scroll down to "Danger Zone"**
6. **Click "Change visibility"** → Select **"Public"**

**Why make it public?** Anyone can pull your Docker images without authentication!

---

## 📚 Step 2: Enable GitHub Pages (For Documentation)

Your documentation will be published to GitHub Pages (not the wiki). Pages is better for auto-deployment and looks more professional!

### Enable Pages:

1. **Go to your repository** → https://github.com/Wikid82/CaddyProxyManagerPlus
2. **Click "Settings"** (top menu)
3. **Click "Pages"** (left sidebar under "Code and automation")
4. **Under "Build and deployment":**
   - **Source**: Select **"GitHub Actions"** (not "Deploy from a branch")
5. That's it! No other settings needed.

Once enabled, your docs will be live at:
```
https://wikid82.github.io/CaddyProxyManagerPlus/
```

**Note:** The first deployment takes 2-3 minutes. Check the Actions tab to see progress!

---

## 🚀 How the Workflows Work

### Docker Build Workflow (`.github/workflows/docker-build.yml`)

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
- `ghcr.io/wikid82/caddyproxymanagerplus:latest`
- `ghcr.io/wikid82/caddyproxymanagerplus:dev`
- `ghcr.io/wikid82/caddyproxymanagerplus:1.0.0`

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

### Test Docker Build:

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

### Test Docs Deployment:

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
   docker pull ghcr.io/wikid82/caddyproxymanagerplus:1.0.0
   docker pull ghcr.io/wikid82/caddyproxymanagerplus:latest
   ```

---

## 🐛 Troubleshooting

### Docker Build Fails

**Problem**: "Error: denied: requested access to the resource is denied"
- **Fix**: Make sure you created the `CPMP_GHCR_TOKEN` secret with the right permissions
- **Check**: Token needs `write:packages` scope
- **Verify**: Settings → Secrets and variables → Actions → Check `CPMP_GHCR_TOKEN` exists

**Problem**: "Error: CPMP_GHCR_TOKEN not found"
- **Fix**: You need to add the secret (see Step 1 above)
- **Double-check**: The secret name is exactly `CPMP_GHCR_TOKEN` (case-sensitive!)

**Problem**: Can't pull the image
- **Fix**: Make the package public (see Step 1 above)
- **Or**: Authenticate with GitHub: `echo $CPMP_GHCR_TOKEN | docker login ghcr.io -u USERNAME --password-stdin`

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
- Open an issue: https://github.com/Wikid82/CaddyProxyManagerPlus/issues
- We're here to help!

---

## 📋 Quick Reference

### Docker Commands
```bash
# Pull latest development version
docker pull ghcr.io/wikid82/caddyproxymanagerplus:dev

# Pull stable version
docker pull ghcr.io/wikid82/caddyproxymanagerplus:latest

# Pull specific version
docker pull ghcr.io/wikid82/caddyproxymanagerplus:1.0.0

# Run the container
docker run -d -p 8080:8080 -v caddy_data:/app/data ghcr.io/wikid82/caddyproxymanagerplus:latest
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

- [ ] Created Personal Access Token with `write:packages` scope
- [ ] Added `CPMP_GHCR_TOKEN` secret to repository
- [ ] GitHub Pages is enabled with "GitHub Actions" source
- [ ] You've tested the Docker build workflow
- [ ] You've tested the docs deployment workflow  
- [ ] Container package is set to "Public" visibility (optional, for easier pulls)
- [ ] Documentation looks good on the published site
- [ ] Docker image runs correctly
- [ ] You've created your first version tag

---

## 🎉 You're Done!

Your CI/CD pipeline is now fully automated! Every time you:
- Push to `main` → New `latest` Docker image + updated docs
- Push to `development` → New `dev` Docker image for testing
- Create a tag → New versioned Docker image

**No manual building needed!** 🚀

<p align="center">
  <em>Questions? Check the <a href="https://docs.github.com/en/actions">GitHub Actions docs</a> or <a href="https://github.com/Wikid82/CaddyProxyManagerPlus/issues">open an issue</a>!</em>
</p>
