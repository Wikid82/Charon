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

### Canonical Release Process (release-please)

Charon uses [`googleapis/release-please-action`](https://github.com/googleapis/release-please-action)
to compute versions, maintain a standing release PR, and cut tags +
GitHub Releases from Conventional Commit history. There is no manual
tagging step and no in-repo version manifest to hand-edit — `.release-please-manifest.json`
is the single source of truth for "what version are we at," kept in
sync by release-please itself.

1. **Commits land on `main`**: every push to `main` runs `.github/workflows/release-please.yml`,
   which walks Conventional Commits since the last release and opens
   or updates a standing `chore(main): release X.Y.Z` pull request.
   If nothing releasable (`feat:`/`fix:`) has landed since the last
   release, no PR is opened or updated.
2. **A human merges the release PR**: merging it is what actually
   ships a release — nothing goes out automatically before that.
3. **On merge, release-please**:
   - Tags the merge commit `vX.Y.Z` (bare, no `charon-` prefix —
     pinned via `include-component-in-tag: false` in
     `release-please-config.json`)
   - Creates the GitHub Release for that tag
4. **Downstream, independently of release-please**:
   - `.github/workflows/orthrus-build.yml` triggers on the new `v*`
     tag and publishes semver-tagged Orthrus agent images
   - The image builds (`docker-build.yml` and `nightly-build.yml`) read the
     `v*` tags with `scripts/generate-changelog.sh` to regenerate the in-app
     "What's New" data (see "What's New Data" below)

5. **Container tags are published** (via `.github/workflows/docker-build.yml`,
   triggered by the push to `main`, not by the tag). On a release commit the
   image gets these tags, with no `v` prefix:

- `X.Y.Z`, for example `0.46.6` (the exact release)
- `X.Y`, for example `0.46` (the newest patch of that minor)
- `latest` (the newest build from `main`)

There is no major-only tag (`0`) while Charon is on 0.x. In SemVer, any 0.x
minor release may contain breaking changes, so a major-only tag would be
misleading.

**Do not manually push `v*` tags.** A manually-created tag desyncs
`.release-please-manifest.json` (which release-please treats as its
source of truth, not live tag state) from the repo's real tag
history — let release-please create every release tag going forward.

`release-please-config.json` sets `skip-changelog: true`, so this
hand-curated `CHANGELOG.md` is never touched by release-please's PRs
or Release generation.

### Deterministic Rollout Verification Gates (Mandatory)

Release sign-off is blocked until all items below pass in the same validation
run.

Enforcement points:

- Release sign-off checklist/process (mandatory): All gates below remain required for release sign-off.
- CI-supported checks (current): `.github/workflows/docker-build.yml` and `.github/workflows/supply-chain-verify.yml` enforce the subset currently implemented in workflows.
- Manual validation required until CI parity: Validate any not-yet-implemented workflow gates via VS Code tasks `Security: Full Supply Chain Audit`, `Security: Verify SBOM`, `Security: Generate SLSA Provenance`, and `Security: Sign with Cosign`.

- [ ] **Digest freshness/parity:** Capture pre-push and post-push index digests
  for the target tag in GHCR and Docker Hub, confirm expected freshness,
  and confirm cross-registry index digest parity.
- [ ] **Per-arch parity:** Confirm per-platform (`linux/amd64`, `linux/arm64`,
  and any published platform) digest parity between GHCR and Docker Hub.
- [ ] **Immutable digest scanning:** Run SBOM and vulnerability scans against
  immutable refs only, using `image@sha256:<index-digest>`.
- [ ] **Artifact freshness:** Confirm scan artifacts are generated after the
  push timestamp and in the same validation run.
- [ ] **Evidence block present:** Include the mandatory evidence block fields
  listed below.

#### Mandatory Evidence Block Fields

- Tag name
- Index digest (`sha256:...`)
- Per-arch digests (platform -> digest)
- Scan tool versions
- Push timestamp and scan timestamp(s)
- Artifact file names generated in this run

## Container Image Tags

### Available Tags

Images are published to GitHub Container Registry (`ghcr.io/wikid82/charon`)
and Docker Hub (`wikid82/charon`) with the same tags.

| Tag | What it is | Updated when |
| --- | --- | --- |
| `latest` | Newest build from `main` (the stable line) | Every push to `main` |
| `X.Y.Z`, e.g. `0.46.6` | An exact release | A release commit on `main` only |
| `X.Y`, e.g. `0.46` | Newest patch of that minor release | A release commit on `main` only |
| `dev` | Newest build from `development` | Every push to `development` |
| `development-<sha7>` | One exact build from `development` | Every push to `development` |
| `main-<sha7>` | One exact build from `main` | Every push to `main` |
| `<sha7>`, e.g. `a1b2c3d` | One exact build from `main` or `development` | Every push to `main` or `development` |
| `nightly` | Newest nightly build | Every nightly build (daily at 02:00 UTC) |
| `nightly-YYYY-MM-DD` | The nightly build from that day | Once per nightly build |
| `nightly-<sha7>` | One exact nightly build | Once per nightly build |
| `pr-<N>` | Build for pull request `N` (same-repository PRs) | Every push to that PR |
| `beta` | Newest build from a pull request into `development` | Every push to such a PR |
| `<branch>-<sha7>` | One exact build from another branch (name simplified to lowercase letters, numbers and dashes) | Every push to that branch |

`<sha7>` is the first seven characters of the commit ID.

There is no `development` tag. Use `dev` for the development line.

Orthrus, the separate agent image, keeps its own `nightly-<sha>` naming and is
not covered by the build version rules below.

### Usage Examples

```bash
# Use latest stable release
docker pull ghcr.io/wikid82/charon:latest

# Use a specific release
docker pull ghcr.io/wikid82/charon:0.46.6

# Use latest nightly build (automated daily at 02:00 UTC)
docker pull ghcr.io/wikid82/charon:nightly

# Use date-specific nightly build
docker pull ghcr.io/wikid82/charon:nightly-2026-01-13

# Use commit-specific nightly build
docker pull ghcr.io/wikid82/charon:nightly-a1b2c3d

# Use development builds (unstable, every commit)
docker pull ghcr.io/wikid82/charon:dev

# Use specific commit from main
docker pull ghcr.io/wikid82/charon:main-a1b2c3d
```

### Nightly Builds

Nightly builds provide a testing ground for features before they reach `main`:

- **Automated**: Built daily at 02:00 UTC from the `nightly` branch
- **Source**: Auto-merged from `development` branch
- **Purpose**: Pre-release testing and validation
- **Stability**: More stable than `development`, less stable than `latest`

**When to use nightly:**

- Testing new features before stable release
- Validating bug fixes
- Contributing to pre-release testing
- Running in staging environments

**When to avoid nightly:**

- Production environments (use `latest` instead)
- Critical infrastructure
- When maximum stability is required

## Build Versions

Every Charon image reports a version on the health endpoint and in the
System Settings update card. The version is the latest release number from
`.release-please-manifest.json` (for example `0.46.5`), plus an optional label
after a `+` that says where the image came from.

| Where the image comes from | Reported version | Example |
| --- | --- | --- |
| Release commit on `main` | `X.Y.Z` | `0.46.6` |
| Other build from `main` | `X.Y.Z` (no label, `main` is the stable line) | `0.46.5` |
| `development` branch | `X.Y.Z+dev.<sha7>` | `0.46.5+dev.a1b2c3d` |
| Pull request | `X.Y.Z+pr.<N>` | `0.46.5+pr.1234` |
| Any other branch | `X.Y.Z+branch.<sha7>` | `0.46.5+branch.a1b2c3d` |
| Nightly build | `X.Y.Z+nightly.<sha7>` | `0.46.5+nightly.a1b2c3d` |
| `make build-versioned` or `make docker-build-versioned` on your own computer | `X.Y.Z+local.<sha7>`, plus `.dirty` when tracked files have uncommitted changes | `0.46.5+local.a1b2c3d.dirty` |
| Plain `docker build` or `go run ./cmd/api` | `dev` (unversioned) | `dev` |

Notes:

- `<sha7>` is the first seven characters of the commit the image was built from.
- Images from a release commit never carry a label, so a release always reports exactly its `X.Y.Z`.
- The `make` targets are described in "Development Builds" below.

### How Versions Are Compared

Charon compares versions by their `X.Y.Z` part only. Anything after a `+` is
ignored, so `0.46.5+dev.a1b2c3d` counts as `0.46.5`.

- The update check offers a newer release only when its `X.Y.Z` is higher than the running build.
- A `dev` build (or any build without a valid version) never shows an update notice.
- Backup restores warn when a backup was made by a newer or older Charon version.

### What's New Version Rules

The "What's New" popup uses the same comparison. A versioned build never shows
changes from a release newer than its own `X.Y.Z`, so an older image never
shows notes for code it does not run.

## Version Information

### Runtime Version Endpoint

```bash
curl http://localhost:8080/api/v1/health
```

Response includes:

```json
{
  "status": "ok",
  "service": "Charon",
  "version": "0.46.5+dev.a1b2c3d",
  "git_commit": "a1b2c3d4e5f6789012345678901234567890abcd",
  "build_time": "2026-01-13T02:00:00Z",
  "internal_ip": "192.168.1.10"
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

Plain local builds stay unversioned (`version=dev`), so they never look like a
release:

```bash
docker build -t charon:dev .
cd backend && go run ./cmd/api
```

To build with the same version label a CI build would get from your checkout,
use the `make` targets. They read the latest release number and add a
`+local.<sha7>` label (plus `.dirty` when tracked files have uncommitted
changes):

```bash
# Backend binary in backend/bin/api
make build-versioned

# Docker image tagged charon:0.46.5-local.a1b2c3d (the + becomes -) and charon:latest
make docker-build-versioned
```

To set a version by hand, pass the build argument yourself:

```bash
docker build \
  --build-arg VERSION=1.2.3 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg VCS_REF=$(git rev-parse HEAD) \
  -t charon:1.2.3 .
```

Notes for local builds:

- The "What's New" data is generated only by CI. A local build embeds the
  committed placeholder (`[]`), so the popup shows nothing until a CI build is used.
  Do not commit a generated `changelog.json`.
- End-to-end tests use an unversioned `dev` image on purpose. The What's New
  test needs that image; it skips itself on a versioned one.

## Changelog Generation

release-please's standing release PR body is generated from commit messages. Use conventional commit format:

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

### What's New Data

The in-app "What's New" popup is built from the same commit history. The image
build generates it and checks it before anything is published. These are the rules:

- There is one entry per release tag (`v*`), newest first.
- On a release commit, the release being built appears even though its tag does not exist yet.
- Merge commits and release process commits (`chore(main): release ...`, promoting nightly) are dropped.
- `feat(security):` and `fix(security):` commits go to the **Security** section. Their subject is shown publicly to every user, so keep it general.
- `feat` and `fix` commits with an internal scope (`deps`, `ci`, `e2e`, `test`, `tests`, `docs`, `build`, `lint`, `release`, `renovate`, `workflow`) are listed under "Show maintenance details", not under New Features or Fixes.
- Every other commit is also listed under maintenance details.
- The build fails rather than shipping an empty or broken list. It checks that the file is valid JSON, has at least one entry, has no merge-commit subjects, and, on a release, has the release as its newest entry.

## CI Tag-based Releases (recommended)

- CI takes the release number from `.release-please-manifest.json`, which
  release-please keeps in step with the `v*` tag it creates. The resolver
  (`scripts/ci/resolve-build-version.sh`) turns that number into the image
  version. See "Build Versions" above.
- release-please creates the tag and the GitHub Release together when its standing release PR is
  merged — see "Canonical Release Process (release-please)" above.
