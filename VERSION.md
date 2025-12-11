# Versioning Guide

## Semantic Versioning

Charon follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR.MINOR.PATCH** (e.g., `1.2.3`)
  - **MAJOR**: Incompatible API changes
  - **MINOR**: New functionality (backward compatible)
  - **PATCH**: Bug fixes (backward compatible)

### Pre-release Identifiers
- `alpha`: Early development, unstable
- `beta`: Feature complete, testing phase
- `rc` (release candidate): Final testing before release

Example: `0.1.0-alpha`, `1.0.0-beta.1`, `2.0.0-rc.2`

## Creating a Release

### Automated Release Process

1. **Update version** in `.version` file:
   ```bash
   echo "1.0.0" > .version
   ```

2. **Commit version bump**:
   ```bash
   git add .version
   git commit -m "chore: bump version to 1.0.0"
   ```

3. **Create and push tag**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

4. **GitHub Actions automatically**:
   - Creates GitHub Release with changelog
   - Builds multi-arch Docker images (amd64, arm64)
   - Publishes to GitHub Container Registry with tags:
     - `v1.0.0` (exact version)
     - `1.0` (minor version)
     - `1` (major version)
     - `latest` (for non-prerelease on main branch)

## Container Image Tags

### Available Tags

- **`latest`**: Latest stable release (main branch)
- **`development`**: Latest development build (development branch)
- **`v1.2.3`**: Specific version tag
- **`1.2`**: Latest patch for minor version
- **`1`**: Latest minor for major version
- **`main-<sha>`**: Commit-specific build from main
- **`development-<sha>`**: Commit-specific build from development

### Usage Examples

```bash
# Use latest stable release
docker pull ghcr.io/wikid82/charon:latest

# Use specific version
docker pull ghcr.io/wikid82/charon:v1.0.0

# Use development builds
docker pull ghcr.io/wikid82/charon:development

# Use specific commit
docker pull ghcr.io/wikid82/charon:main-abc123
```

## Version Information

### Runtime Version Endpoint

```bash
curl http://localhost:8080/api/v1/health
```

Response includes:
```json
{
  "status": "ok",
  "service": "charon",
  "version": "1.0.0",
  "git_commit": "abc1234567890def",
  "build_date": "2025-11-17T12:34:56Z"
}
```

### Container Image Labels

View version metadata:
```bash
docker inspect ghcr.io/wikid82/charon:latest \
  --format='{{json .Config.Labels}}' | jq
```

Returns OCI-compliant labels:
- `org.opencontainers.image.version`
- `org.opencontainers.image.created`
- `org.opencontainers.image.revision`
- `org.opencontainers.image.source`

## Development Builds

Local builds default to `version=dev`:
```bash
docker build -t charon:dev .
```

Build with custom version:
```bash
docker build \
  --build-arg VERSION=1.2.3 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg VCS_REF=$(git rev-parse HEAD) \
  -t charon:1.2.3 .
```

## Changelog Generation

The release workflow automatically generates changelogs from commit messages. Use conventional commit format:

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `chore:` Maintenance tasks
- `refactor:` Code refactoring
- `test:` Test updates
- `ci:` CI/CD changes

Example:
```bash
git commit -m "feat: add TLS certificate management"
git commit -m "fix: correct proxy timeout handling"
```

## CI Tag-based Releases (recommended)

- CI derives the release `Version` from the Git tag (e.g., `v1.2.3`) and embeds this value into the backend binary via Go ldflags; frontend reads the version from the backend's API. This avoids automatic commits to `main`.
- The `.version` file is optional. If present, use the `scripts/check-version-match-tag.sh` script or the included pre-commit hook to validate that `.version` matches the latest Git tag.
- CI will still generate changelogs automatically using the release-drafter workflow and create GitHub Releases when tags are pushed.
