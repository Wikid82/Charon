# Plan: Fix Supply Chain Vulnerability Reporting

## Objective
Fix the `supply-chain-pr.yml` workflow where PR comments report 0 vulnerabilities despite known CVEs, and ensure the workflow correctly fails on critical vulnerabilities.

## Context
The current workflow uses `anchore/scan-action` to scan for vulnerabilities. However, there are potential issues with:
1.  **Output File Handling:** The workflow assumes `results.json` is created, but `anchore/scan-action` with `output-format: json` might not produce this file by default without an explicit `output-file` parameter or capturing output.
2.  **Parsing Logic:** If the file is missing, the `jq` parsing gracefully falls back to 0, masking the error.
3.  **Failure Condition:** The failure step references `${{ steps.grype-scan.outputs.critical_count }}`, which likely does not exist on the `anchore/scan-action` step. It should reference the calculated output from the parsing step.

## Research & Diagnosis Steps

### 1. Debug Output paths
We need to verify if `results.json` is actually generated.
-   **Action:** Add a step to list files in the workspace immediately after the scan.
-   **Action:** Add a debug `cat` of the results file if it exists, or header of it.

### 2. Verify `anchore/scan-action` behavior
The `anchore/scan-action` (v7.3.2) documentation suggests that `output-format` is used, but typically it defaults to `results.[format]`. However, explicit `output-file` prevents ambiguity.

## Implementation Plan

### Phase 1: Robust Path & Debugging
1.  **Explicit Output File:** Modify the `anchore/scan-action` step to explicitly set `output-format: json` AND likely we should try to rely on the default behavior but *check* it.
    *Actually, better practice:* The action supports `output-format` as a list. If we want a file, we usually just look for it.
    *Correction:* We will explicitly check for the file and fail if missing, rather than defaulting to 0.
2.  **List Files:** Add `ls -la` after scan to see exactly what files are created.

### Phase 2: Fix Logic Errors
1.  **Update "Fail on critical vulnerabilities" step**:
    -   Change `${{ steps.grype-scan.outputs.critical_count }}` to `${{ steps.vuln-summary.outputs.critical_count }}`.
2.  **Robust `jq` parsing**:
    -   In `Process vulnerability results`, explicitly check for existence of `results.json` (or whatever the action outputs).
    -   If missing, **EXIT 1** instead of setting counts to 0. This forces us to fix the path issue rather than silently passing.
    -   Use `tee` or `cat` to print the first few lines of the JSON to stdout for debugging logs.

### Phase 3: Validation
1.  Run the workflow on a PR (or simulate via push).
2.  Verify the PR comment shows actual numbers.
3.  Verify the workflow fails if critical vulnerabilities are found (or we can lower the threshold to test).

## Detailed Changes

### `supply-chain-pr.yml`

```yaml
      # ... inside steps ...

      - name: Scan for vulnerabilities
        if: steps.set-target.outputs.image_name != ''
        uses: anchore/scan-action@7037fa011853d5a11690026fb85feee79f4c946c # v7.3.2
        id: grype-scan
        with:
          sbom: sbom.cyclonedx.json
          fail-build: false
          output-format: json
          # We might need explicit output selection implies asking for 'json' creates 'results.json'

      - name: Debug Output Files
        if: steps.set-target.outputs.image_name != ''
        run: |
            echo "📂 Listing workspace files:"
            ls -la

      - name: Process vulnerability results
        if: steps.set-target.outputs.image_name != ''
        id: vuln-summary
        run: |
          # The scan-action output behavior verification
          JSON_RESULT="results.json"
          SARIF_RESULT="results.sarif"

          # [NEW] Check if scan actually produced output
          if [[ ! -f "$JSON_RESULT" ]]; then
             echo "❌ Error: $JSON_RESULT not found!"
             echo "Available files:"
             ls -la
             exit 1
          fi

          mv "$JSON_RESULT" grype-results.json

          # Debug content (head)
          echo "📄 Grype JSON Preview:"
          head -n 20 grype-results.json

          # ... existing renaming for sarif ...

          # ... existing jq logic, but remove 'else' block for missing file since we exit above ...

      # ...

      - name: Fail on critical vulnerabilities
        if: steps.set-target.outputs.image_name != ''
        run: |
          # [FIX] Use the output from the summary step, NOT the scan step
          CRITICAL_COUNT="${{ steps.vuln-summary.outputs.critical_count }}"

          if [[ "${CRITICAL_COUNT}" -gt 0 ]]; then
            echo "🚨 Found ${CRITICAL_COUNT} CRITICAL vulnerabilities!"
            echo "Please review the vulnerability report and address critical issues before merging."
            exit 1
          fi
```

### Acceptance Criteria
-   [ ] Workflow "Fail on critical vulnerabilities" uses `steps.vuln-summary.outputs.critical_count`.
-   [ ] `Process vulnerability results` step fails if the scan output file is missing.
-   [ ] Debug logging (ls -la) is present to confirm file placement.
