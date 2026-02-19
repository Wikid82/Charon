# Codecov Patch Coverage Fix Plan: `user_handler.go` (AcceptInvite constant-time branch)

## Problem Statement

Codecov patch coverage is failing at **66.66667%** due to **3 uncovered patch lines** in:

- `backend/internal/api/handlers/user_handler.go`

Codecov reports **2 lines missing** and **1 line partially covered**.

## What’s Uncovered (Exact Lines + Behavior)

The uncovered patch lines are in `(*UserHandler).AcceptInvite`:

- **Partial line:** the branch condition
  - `if !util.ConstantTimeCompare(user.InviteToken, req.Token) {`
- **Missing lines:** the branch body
  - `c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid invite token"})`
  - `return`

### What behavior this represents

This branch is intended as defense-in-depth: if the stored invite token and the request token differ, reject the request with **HTTP 401**.

## Root Cause Analysis (Hypothesis)

These lines are currently **effectively unreachable** under normal execution:

1. The handler looks up the user via `WHERE invite_token = req.Token`.
2. If a row is found, `user.InviteToken` should already equal `req.Token`.
3. Therefore, `!util.ConstantTimeCompare(user.InviteToken, req.Token)` will never be true, leaving its body uncovered.

This yields the exact Codecov pattern we see:

- the `if` line is hit (partial)
- its body is never hit (2 lines missing)

## Existing Test Harness (Repo Patterns)

### Handler tests (unit)

The repository already has strong patterns for handler unit tests:

- Use Gin in test mode: `gin.SetMode(gin.TestMode)`
- Build a minimal router: `r := gin.New()`
- Register a single route per test: `r.POST("/invite/accept", handler.AcceptInvite)`
- Use `httptest.NewRecorder()` + `httptest.NewRequest(...)`
- Use SQLite in-memory via GORM for isolated tests

Relevant existing files:

- `backend/internal/api/handlers/user_handler_test.go` (many UserHandler endpoint tests)
- `backend/internal/api/handlers/user_handler_coverage_test.go` (targeted “error/coverage” tests using `gin.CreateTestContext`)
- `backend/internal/api/handlers/testdb_test.go` (shared `OpenTestDB*` helpers used across handler tests)

## Minimal Fix Strategy (Supervisor-Recommended)

Because the DB query already enforces `invite_token = req.Token`, the constant-time compare mismatch branch in `AcceptInvite` is **unreachable**. The smallest, behavior-preserving fix (and the one that avoids creating a response oracle) is to **delete the dead mismatch branch** rather than trying to make it testable.

### Why this is safe

- If the token doesn’t match, the DB lookup already fails and returns **404 Not Found** (`"Invalid or expired invite token"`).
- Keeping a separate **401 Unauthorized** path for a mismatch (even if unreachable today) risks future “oracle” behavior differences.
- Removing the branch is a strict simplification: it does not introduce any new acceptance conditions.

## Proposed Code Change (Minimal, Behavior-Preserving)

### Edit

- `backend/internal/api/handlers/user_handler.go`

### Change

In `(*UserHandler).AcceptInvite`, remove the redundant constant-time compare mismatch block (currently the only `util.` usage in this file):

- Remove the comment and `if !util.ConstantTimeCompare(user.InviteToken, req.Token) { ... }` block at lines ~827–834 (the `if` is at line 830).
- Remove the now-unused import `github.com/Wikid82/charon/backend/internal/util` (line 18) if it becomes unused after deleting the block.

## Unit Tests

No new unit tests are expected/required for this change because it only deletes unreachable code.

If any existing tests assert a **401** for invite-token mismatch (none are expected), update those tests to assert the existing invalid-token behavior (**404**) instead.

## Verification Steps

1. Run VS Code task: **Test: Backend with Coverage**
2. Run VS Code task: **Build: Backend** (or equivalent: `cd backend && go build ./...`)
3. (Optional but recommended) Run: **Lint: Go Vet**
4. Confirm Codecov/coverage gate remains **>= 85%** and the patch no longer reports uncovered lines for the removed branch.

## Security Note (Avoiding an Oracle)

This approach intentionally avoids distinguishing “token exists but mismatched” vs “token not found” via different status codes/messages, which can become a token-validity oracle. After the change, invalid tokens continue to get the existing **404** response path.

## Acceptance Criteria

- Codecov patch coverage for the change is **>= 85%**
- No uncovered lines remain in `backend/internal/api/handlers/user_handler.go` (specifically the `ConstantTimeCompare` mismatch branch)
- `Test: Backend with Coverage` passes
