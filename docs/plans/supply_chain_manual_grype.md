# Plan: Replace Anchore Scan Action with Manual Grype Execution

## 1. Introduction
The `anchore/scan-action` has been unreliable in producing the expected output files (`results.json`) in our PR workflow, causing downstream failures in the vulnerability processing step. To ensure reliability and control over the output, we will replace the pre-packaged action with a manual installation and execution of the `grype` binary.

## 2. Technical Specifications
### Target File
- `.github/workflows/supply-chain-pr.yml`

### Changes
1.  **Replace** the step named "Scan for vulnerabilities".
    -   **Current**: Uses `anchore/scan-action`.
    -   **New**: Uses a shell script to install a pinned version of `grype` (e.g., `v0.77.0`) and run it twice (once for JSON, once for SARIF).
    -   **Why**: Direct shell redirection (`>`) guarantees the file is created where we expect it, avoiding the "silent failure" behavior of the action. Using a pinned version ensures reproducibility and stability.

2.  **Update** the step named "Process vulnerability results".
    -   **Current**: Looks for `results.json` and renames it to `grype-results.json`.
    -   **New**: Checks directly for `grype-results.json` (since we produced it directly).

## 3. Implementation Plan

### Step 1: Replace "Scan for vulnerabilities"
Replace the existing `anchore/scan-action` step with the following shell script. Note the explicit version pinning for `grype`.

```yaml
      - name: Scan for vulnerabilities (Manual Grype)
        if: steps.set-target.outputs.image_name != ''
        id: grype-scan
        run: |
          set -e
          echo "⬇️ Installing Grype (v0.77.0)..."
          curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin v0.77.0

          echo "🔍 Scanning SBOM for vulnerabilities..."

          # Generate JSON output
          echo "📄 Generating JSON report..."
          grype sbom:sbom.cyclonedx.json -o json > grype-results.json

          # Generate SARIF output (for GitHub Security tab)
          echo "📄 Generating SARIF report..."
          grype sbom:sbom.cyclonedx.json -o sarif > grype-results.sarif

          echo "✅ Scan complete. Output files generated:"
          ls -lh grype-results.*
```

### Step 2: Update "Process vulnerability results"
Modify the processing step to remove the file renaming logic, as the files are already in the correct format.

```yaml
      - name: Process vulnerability results
        if: steps.set-target.outputs.image_name != ''
        id: vuln-summary
        run: |
          JSON_RESULT="grype-results.json"

          # Verify scan actually produced output
          if [[ ! -f "$JSON_RESULT" ]]; then
             echo "❌ Error: $JSON_RESULT not found!"
             echo "Available files:"
             ls -la
             exit 1
          fi

          # Debug content (head)
          echo "📄 Grype JSON Preview:"
          head -n 20 "$JSON_RESULT"

          # Count vulnerabilities by severity
          CRITICAL_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Critical")] | length' "$JSON_RESULT" 2>/dev/null || echo "0")
          HIGH_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "High")] | length' "$JSON_RESULT" 2>/dev/null || echo "0")
          MEDIUM_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Medium")] | length' "$JSON_RESULT" 2>/dev/null || echo "0")
          LOW_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Low")] | length' "$JSON_RESULT" 2>/dev/null || echo "0")
          TOTAL_COUNT=$(jq '.matches | length' "$JSON_RESULT" 2>/dev/null || echo "0")

          echo "critical_count=${CRITICAL_COUNT}" >> "$GITHUB_OUTPUT"
          echo "high_count=${HIGH_COUNT}" >> "$GITHUB_OUTPUT"
          echo "medium_count=${MEDIUM_COUNT}" >> "$GITHUB_OUTPUT"
          echo "low_count=${LOW_COUNT}" >> "$GITHUB_OUTPUT"
          echo "total_count=${TOTAL_COUNT}" >> "$GITHUB_OUTPUT"

          echo "📊 Vulnerability Summary:"
          echo "  Critical: ${CRITICAL_COUNT}"
          echo "  High: ${HIGH_COUNT}"
          echo "  Medium: ${MEDIUM_COUNT}"
          echo "  Low: ${LOW_COUNT}"
          echo "  Total: ${TOTAL_COUNT}"
```

## 4. Verification
1.  Commit the changes to a new branch.
2.  The workflow should trigger automatically on push (since we are modifying the workflow or pushing to a branch).
3.  Verify the "Scan for vulnerabilities (Manual Grype)" step runs successfully and installs the specified version.
4.  Verify the "Process vulnerability results" step correctly reads the `grype-results.json`.
