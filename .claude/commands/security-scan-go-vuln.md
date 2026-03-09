# Security: Go Vulnerability Scan

Run `govulncheck` to detect known vulnerabilities in Go dependencies.

## Command

```bash
.github/skills/scripts/skill-runner.sh security-scan-go-vuln
```

## Direct Alternative

```bash
cd backend && govulncheck ./...
```

## What It Does

`govulncheck` scans your Go module graph against the Go vulnerability database (vuln.go.dev). Unlike Trivy, it:
- Only reports vulnerabilities in code paths that are **actually called** (not just imported)
- Reduces false positives significantly
- Is the authoritative source for Go-specific CVEs

## Install govulncheck

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## On Findings

For each finding:
1. Check if the vulnerable function is actually called in Charon's code
2. If called: update the dependency immediately
3. If not called: document why it's not a risk (govulncheck may still flag it)

Use `/supply-chain-remediation` for the full remediation workflow:
```bash
go get affected-package@fixed-version
go mod tidy && go mod verify
```

## Related

- `/security-scan-trivy` — Broader dependency and image scan
- `/security-scan-docker-image` — Post-build image vulnerability scan
- `/supply-chain-remediation` — Fix vulnerabilities
