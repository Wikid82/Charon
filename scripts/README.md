# Scripts Directory

## Running Tests Locally Before Pushing to CI

### WAF Integration Test

**Always run this locally before pushing WAF-related changes to avoid CI failures:**

```bash
# From project root
bash ./scripts/coraza_integration.sh
```

Or use the VS Code task: `Ctrl+Shift+P` → `Tasks: Run Task` → `Coraza: Run Integration Script`

**Requirements:**

- Docker image `charon:local` must be built first:

  ```bash
  docker build -t charon:local .
  ```

- The script will:
  1. Start a test container with WAF enabled
  2. Create a backend container (httpbin)
  3. Test WAF in block mode (expect HTTP 403)
  4. Test WAF in monitor mode (expect HTTP 200)
  5. Clean up all test containers

**Expected output:**

```text
✓ httpbin backend is ready
✓ Coraza WAF blocked payload as expected (HTTP 403) in BLOCK mode
✓ Coraza WAF in MONITOR mode allowed payload through (HTTP 200) as expected
=== All Coraza integration tests passed ===
```

### Other Test Scripts

- **Security Scan**: `bash ./scripts/security-scan.sh`
- **Go Test Coverage**: `bash ./scripts/go-test-coverage.sh`
- **Frontend Test Coverage**: `bash ./scripts/frontend-test-coverage.sh`

## What's New and Build Version Scripts (CI)

These scripts make the image build decide the What's New data and the version
label. CI runs them; you can run them by hand to check a change before pushing.
Every script prints its usage with `--help`, and its exit codes are listed in
the header comment at the top of the file.

| Script | Used by | What it does |
| --- | --- | --- |
| `generate-changelog.sh` | `docker-build.yml` (setup), `nightly-build.yml` | Writes `backend/internal/changelog/data/changelog.json` from git history. `--current-version X.Y.Z` adds the release being built. `--require-entries` fails when there are no entries. |
| `ci/check-changelog-data.sh` | Both image builds, and the quality checks | Fails when the data file is missing, malformed, has merge-commit subjects, or (with `--expect-newest`) does not have the expected newest release. |
| `ci/detect-release-commit.sh` | `docker-build.yml` (setup) | Decides whether the commit is a release commit (on `main`, manifest version went up). Prints `is_release` and `version`. |
| `ci/resolve-build-version.sh` | `docker-build.yml`, `nightly-build.yml`, `Makefile` | Prints the version an image reports, such as `0.46.5+pr.1234`. The channel is set with `--channel`. |
| `ci/smoke-whats-new.sh` | `docker-build.yml` (before publishing), `nightly-build.yml` | Starts a built image on a temporary volume and checks its version, the What's New list and the new-user popup rules. Needs `docker`, `curl` and `jq`. |

Shell tests for these scripts use [bats](https://github.com/bats-core/bats-core)
and live in `scripts/tests/`. Run them and the linter from the repository root:

```bash
bats scripts/tests/generate-changelog.bats scripts/tests/check-changelog-data.bats \
  scripts/tests/detect-release-commit.bats scripts/tests/resolve-build-version.bats \
  scripts/tests/smoke-whats-new.bats
shellcheck scripts/generate-changelog.sh scripts/ci/*.sh
```

The smoke script needs a running Docker daemon and a published or PR image.
See `docs/issues/whats-new-changelog-manual-test-plan.md` for the manual steps.

## CI/CD Workflows

Changes to these scripts may trigger CI workflows:

- `coraza_integration.sh` → WAF Integration Tests workflow
- Files in `.github/workflows/` directory control CI behavior

**Tip**: Run tests locally to save CI minutes and catch issues faster!
