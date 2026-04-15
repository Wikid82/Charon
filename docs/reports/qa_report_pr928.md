# QA Audit Report — PR #928: CI Test Fix

**Date:** 2026-04-13
**Scope:** Targeted fix for two CI test failures across three files
**Auditor:** QA Security Agent

## Modified Files

| File | Change |
|---|---|
| `backend/internal/api/handlers/certificate_handler.go` | Added `key_file` validation guard for non-PFX uploads |
| `backend/internal/api/handlers/certificate_handler_test.go` | Added `TestCertificateHandler_Upload_MissingKeyFile_MultipartWithCert` test |
| `backend/internal/services/certificate_service_coverage_test.go` | Removed duplicate `ALTER TABLE` in `TestCertificateService_MigratePrivateKeys` |

## Check Results

| # | Check | Result | Details |
|---|---|---|---|
| 1 | **Backend Build** (`go build ./...`) | **PASS** | Clean build, no errors |
| 2a | **Targeted Test 1** (`TestCertificateHandler_Upload_MissingKeyFile_MultipartWithCert`) | **PASS** | Previously failing, now passes |
| 2b | **Targeted Test 2** (`TestCertificateService_MigratePrivateKeys`) | **PASS** | Previously failing (duplicate ALTER TABLE), now passes — all 3 subtests pass |
| 3a | **Regression: Handler Suite** (`TestCertificateHandler*`, 37 tests) | **PASS** | All 37 tests pass in 1.27s |
| 3b | **Regression: Service Suite** (`TestCertificateService*`) | **PASS** | All tests pass in 2.96s |
| 4 | **Go Vet** (`go vet ./...`) | **PASS** | No issues |
| 5 | **Golangci-lint** (handlers + services) | **PASS** | All warnings are pre-existing in unmodified files (crowdsec, audit_log). Zero new lint issues in modified files |
| 6 | **Security Review** | **PASS** | See analysis below |

## Security Analysis

The handler change (lines 169–175 of `certificate_handler.go`) was reviewed for:

| Vector | Assessment |
|---|---|
| **Injection** | `services.DetectFormat()` operates on already-read `certBytes` (bounded by `maxFileSize` = 1MB via `io.LimitReader`). No additional I/O or shell invocation. |
| **Information Disclosure** | Returns a static error string `"key_file is required for PEM/DER certificate uploads"`. No user-controlled data reflected in the response. |
| **Auth Bypass** | Route `POST /certificates` is registered inside the `management` group (confirmed at `routes.go:696`), which requires authentication. The guard is additive — it rejects earlier, not later. |
| **DoS** | No new allocations or expensive operations. `DetectFormat` is a simple byte-header check on data already in memory. |

**Verdict:** No new attack surface introduced. The change is a pure input validation tightening.

## Warnings

- **Pre-existing lint warnings** exist in unmodified files (`crowdsec_handler.go`, `crowdsec_*_test.go`, `audit_log_handler_test.go`). These are tracked separately and are not related to this PR.

## Final Verdict

**PASS** — All six checks pass. Both previously failing CI tests now succeed. No regressions detected in the broader handler and service suites. No security concerns with the changes.
