# Security: GORM Security Scan

Run the Charon GORM security scanner to detect SQL injection risks and unsafe GORM usage patterns.

## When to Run

**MANDATORY** when any of the following changed:
- `backend/internal/models/**`
- GORM service files
- Database migration code
- Any file with `.db.`, `.Where(`, `.Raw(`, or `.Exec(` calls

## Command

```bash
.github/skills/scripts/skill-runner.sh security-scan-gorm
```

## Direct Alternative (Check Mode — Blocks on Findings)

```bash
./scripts/scan-gorm-security.sh --check
```

Check mode exits non-zero if any CRITICAL or HIGH findings are present. This is the mode used in the DoD gate.

## What It Detects

- Raw SQL with string concatenation (SQL injection risk)
- Unparameterized dynamic queries
- Missing input validation before DB operations
- Unsafe use of `db.Exec()` with user input
- Patterns that bypass GORM's built-in safety mechanisms

## On Findings

All CRITICAL and HIGH findings must be fixed before the task is considered done. Do not accept the task completion from any agent until this passes.

See `.github/skills/.skill-quickref-gorm-scanner.md` for remediation patterns.

## Related

- `/sql-code-review` — Manual SQL/GORM code review
- `/security-scan-trivy` — Dependency vulnerability scan
