1) Our coverage patch is still lacking tests for the new functionality we added in the last sprint. We need to write unit tests to ensure that all edge cases are covered.

<https://github.com/Wikid82/Charon/pull/461#issuecomment-3719387466>

Codecov Report
❌ Patch coverage is 80.00000% with 7 lines in your changes missing coverage. Please review.

Files with missing lines Patch % Lines
...ackend/internal/api/handlers/encryption_handler.go 60.00% 4 Missing and 2 partials ⚠️
backend/internal/api/handlers/import_handler.go 50.00% 1 Missing ⚠️

1) Our latest push or the renevator updates has introduced some vulnerabilities that were not present before. We need to investigate and fix these vulnerabilities.
    - If they are in third-party dependencies, we should consider updating or replacing those dependencies. If they are recent versions we need to comment on the supply chain PR comment as to why we are accepting the risk / waiting for updates. <https://github.com/Wikid82/Charon/pull/461#issuecomment-3746737390>
    - If they are in our own code, we need to patch them immediately.

    Status: ✅ PASSED
Commit: 69f7498
Image: ghcr.io/wikid82/charon:pr-461
Components Scanned: 755

📊 Vulnerability Summary
Severity Count
🔴 Critical 0
🟠 High 0
🟡 Medium 8
🟢 Low 1
📋 View Full Report
📦 Download Artifacts
