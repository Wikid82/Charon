# Automated Semantic Versioning - Implementation Summary

## Overview
Added comprehensive automated semantic versioning to CaddyProxyManager+ with version injection into container images, runtime version endpoints, and automated release workflows.

## Components Implemented

### 1. Dockerfile Version Injection
**File**: `Dockerfile`
- Added build arguments: `VERSION`, `BUILD_DATE`, `VCS_REF`
- Backend builder injects version info via Go ldflags during compilation
- Final image includes OCI-compliant labels for version metadata
- Version defaults to `dev` for local builds

### 2. Runtime Version Package
**File**: `backend/internal/version/version.go`
- Added `GitCommit` and `BuildDate` variables (injected via ldflags)
- Added `Full()` function returning complete version string
- Version information available at runtime via `/api/v1/health` endpoint

### 3. Health Endpoint Enhancement
**File**: `backend/internal/api/handlers/health_handler.go`
- Extended to expose version metadata:
  - `version`: Semantic version (e.g., "1.0.0")
  - `git_commit`: Git commit SHA
  - `build_date`: Build timestamp

### 4. Docker Publishing Workflow
**File**: `.github/workflows/docker-publish.yml`
- Added `workflow_call` trigger for reusability
- Uses `docker/metadata-action` for automated tag generation
- Tag strategy:
  - `latest` for main branch
  - `development` for development branch
  - `v1.2.3`, `1.2`, `1` for semantic version tags
  - `{branch}-{sha}` for commit-specific builds
- Passes version metadata as build args

### 5. Release Workflow
**File**: `.github/workflows/release.yml`
- Triggered on `v*.*.*` tags
- Automatically generates changelog from commit messages
- Creates GitHub Release (marks pre-releases for alpha/beta/rc)
- Calls docker-publish workflow to build and publish images

### 6. Release Helper Script
**File**: `scripts/release.sh`
- Interactive script for creating releases
- Validates semantic version format
- Updates `.version` file
- Creates annotated git tag
- Pushes to remote and triggers workflows
- Safety checks: uncommitted changes, duplicate tags

### 7. Version File
**File**: `.version`
- Single source of truth for current version
- Current: `0.1.0-alpha`
- Used by release script and Makefile

### 8. Documentation
**File**: `VERSION.md`
- Complete versioning guide
- Release process documentation
- Container image tag reference
- Examples for all version query methods

### 9. Build System Updates
**File**: `Makefile`
- Added `docker-build-versioned`: Builds with version from `.version` file
- Added `release`: Interactive release creation
- Updated help text

**File**: `.gitignore`
- Added `CHANGELOG.txt` to ignored files

## Usage Examples

### Creating a Release
```bash
# Interactive release
make release

# Manual release
echo "1.0.0" > .version
git add .version
git commit -m "chore: bump version to 1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main
git push origin v1.0.0
```

### Building with Version
```bash
# Using Makefile (reads from .version)
make docker-build-versioned

# Manual with custom version
docker build \
  --build-arg VERSION=1.2.3 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg VCS_REF=$(git rev-parse HEAD) \
  -t cpmp:1.2.3 .
```

### Querying Version at Runtime
```bash
# Health endpoint includes version
curl http://localhost:8080/api/v1/health
{
  "status": "ok",
  "service": "CPMP",
  "version": "1.0.0",
  "git_commit": "abc1234567890def",
  "build_date": "2025-11-17T12:34:56Z"
}

# Container image labels
docker inspect ghcr.io/wikid82/cpmp:latest \
  --format='{{json .Config.Labels}}' | jq
```

## Automated Workflows

### On Tag Push (v1.2.3)
1. Release workflow creates GitHub Release with changelog
2. Docker publish workflow builds multi-arch images (amd64, arm64)
3. Images tagged: `v1.2.3`, `1.2`, `1`, `latest` (if main)
4. Published to GitHub Container Registry

### On Branch Push
1. Docker publish workflow builds images
2. Images tagged: `development` or `main-{sha}`
3. Published to GHCR (not for PRs)

## Benefits

1. **Traceability**: Every container image traceable to exact git commit
2. **Automation**: Zero-touch release process after tag push
3. **Flexibility**: Multiple tag strategies (latest, semver, commit-specific)
4. **Standards**: OCI-compliant image labels
5. **Runtime Discovery**: Version queryable via API endpoint
6. **User Experience**: Clear version information for support/debugging

## Testing

Version injection tested and working:
- ✅ Go binary builds with ldflags injection
- ✅ Health endpoint returns version info
- ✅ Dockerfile ARGs properly scoped
- ✅ OCI labels properly set
- ✅ Release script validates input
- ✅ Workflows configured correctly

## Next Steps

1. Test full release workflow with actual tag push
2. Consider adding `/api/v1/version` dedicated endpoint
3. Display version in frontend UI footer
4. Add version to error reports/logs
5. Document version strategy in contributor guide
