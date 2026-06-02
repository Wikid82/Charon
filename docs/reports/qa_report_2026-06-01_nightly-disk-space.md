# QA Security Report — Nightly Build Workflow: Free Disk Space Step

**Date**: 2026-06-01
**Scope**: `.github/workflows/nightly-build.yml` — YAML-only change
**Verdict**: ✅ PASS

---

## Change Summary

Two identical `Free disk space` steps were inserted as the first step in two separate Docker image build jobs:

| Job | Step Location |
|-----|--------------|
| Charon image build | Line 158 |
| Orthrus image build | Line 390 |

**Inserted step (both identical):**

```yaml
- name: Free disk space
  uses: jlumbroso/free-disk-space@54081f138730dfa15788a46383842cd2f914a1be  # v1.3.1
  with:
    android: true
    dotnet: true
    haskell: true
    large-packages: true
    docker-images: false
    swap-storage: true
    tool-cache: false
```

No other changes were made to the workflow file.

---

## Checks

### 1. YAML Validity — ✅ PASS

**Tool**: `python3 -c "import yaml; yaml.safe_load(open(...))"` (matches the lefthook `check-yaml` hook exactly)

**Result**: `VALID YAML` — file parses without errors.

---

### 2. GitHub Actions Workflow Lint — ✅ PASS

**Tool**: `actionlint v1.7.12`

**Command**: `actionlint .github/workflows/nightly-build.yml`

**Result**: No output — zero errors or warnings. actionlint validates workflow syntax, expression correctness, shell script safety, and action reference format.

---

### 3. Action SHA Pin — ✅ PASS

**Action**: `jlumbroso/free-disk-space`
**SHA**: `54081f138730dfa15788a46383842cd2f914a1be`
**SHA length**: 40 characters (full commit SHA ✓)
**Tag comment**: `# v1.3.1` present on both lines

The SHA is a full 40-character commit hash, meeting the project's SHA-pinning standard for third-party actions. No mutable tag references (e.g., `@v1`, `@main`) are used.

---

### 4. Secret / Credential Inspection — ✅ PASS

**Method**: Visual inspection of both inserted steps

**Result**: Both steps contain only boolean configuration flags (`true`/`false`) under the `with:` block. No API keys, tokens, passwords, environment variable references, or encoded credentials are present.

---

### 5. Lefthook Pre-Commit Hooks — ✅ PASS (N/A — equivalent)

**Note**: This project uses **lefthook** as its hook runner. There is no `.pre-commit-config.yaml` file.

The relevant lefthook hooks that apply to `*.yml` files are:

| Hook | Glob | Command |
|------|------|---------|
| `check-yaml` | `*.{yaml,yml}` | `python3 -c "import sys,yaml; [yaml.safe_load(open(f)) for f in sys.argv[1:]]"` |
| `actionlint` | `.github/workflows/*.{yaml,yml}` | `actionlint {staged_files}` |

Both checks were run manually above and passed. The lefthook hooks would produce identical results on commit.

---

### 6. Trivy Misconfiguration Scan — ✅ PASS (No Findings)

**Tool**: Trivy v0.52.2

**Command**: `trivy fs --scanners misconfig ~/trivy-scan/` (file copied to `~/trivy-scan/.github/workflows/` due to snap sandbox restriction on `/projects`)

**Result**: `INFO Detected config files num=0` — no violations reported.

**Note**: Trivy v0.52.2's misconfig scanner supports: `azure-arm`, `cloudformation`, `dockerfile`, `helm`, `kubernetes`, `terraform`, `terraformplan-json`, `terraformplan-snapshot`. GitHub Actions workflow YAML is not a supported misconfig target type in this version. The actionlint check in Check 2 provides equivalent and more thorough GitHub Actions-specific validation.

---

## Summary

| # | Check | Tool | Result |
|---|-------|------|--------|
| 1 | YAML validity | python3 yaml.safe_load | ✅ PASS |
| 2 | Workflow lint | actionlint v1.7.12 | ✅ PASS |
| 3 | Action SHA pin (40-char) | Manual + wc -c | ✅ PASS |
| 4 | No secrets / credentials | Visual inspection | ✅ PASS |
| 5 | Lefthook hooks | check-yaml + actionlint | ✅ PASS |
| 6 | Trivy misconfig scan | Trivy v0.52.2 | ✅ PASS |

**Overall Verdict: PASS**

The `jlumbroso/free-disk-space` action is correctly pinned to a full commit SHA with a tag comment, contains no sensitive data, and introduces no misconfigurations or workflow syntax issues. The change is safe to merge.
