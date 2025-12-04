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
```
✓ httpbin backend is ready
✓ Coraza WAF blocked payload as expected (HTTP 403) in BLOCK mode
✓ Coraza WAF in MONITOR mode allowed payload through (HTTP 200) as expected
=== All Coraza integration tests passed ===
```

### Other Test Scripts

- **Security Scan**: `bash ./scripts/security-scan.sh`
- **Go Test Coverage**: `bash ./scripts/go-test-coverage.sh`
- **Frontend Test Coverage**: `bash ./scripts/frontend-test-coverage.sh`

## CI/CD Workflows

Changes to these scripts may trigger CI workflows:
- `coraza_integration.sh` → WAF Integration Tests workflow
- Files in `.github/workflows/` directory control CI behavior

**Tip**: Run tests locally to save CI minutes and catch issues faster!
