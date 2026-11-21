# Utility: Update Go Version

Updates the local Go installation to match the version specified in `go.work`.

## Purpose

When Renovate bot updates the Go version in `go.work`, this skill automatically downloads and installs the matching Go version locally.

## Usage

```bash
.github/skills/scripts/skill-runner.sh utility-update-go-version
```

## What It Does

1. Reads the required Go version from `go.work`
2. Compares against the currently installed version
3. If different, downloads and installs the new version using `golang.org/dl`
4. Updates the system symlink to point to the new version

## When to Use

- After Renovate bot creates a PR updating `go.work`
- When you see "packages.Load error: go.work requires go >= X.Y.Z"
- Before building if you get Go version mismatch errors

## Requirements

- `sudo` access (for updating symlink)
- Internet connection (for downloading Go SDK)
