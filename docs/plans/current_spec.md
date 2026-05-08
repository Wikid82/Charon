# CI Fix — Vitest `invites a new user` Failure

**Branch**: `fix/invite-user-test-jsdom`
**PR**: Targets `development`
**Date**: 2026-05-08

---

> **Archived**: The previous spec (CI Fix — Grep Pattern Bug & Orthrus Test Cleanup) has been
> superseded. This document covers the active Vitest CI failure.

---

## 1. Introduction

### Overview

One Vitest test is failing in CI:

```
FAIL  src/pages/__tests__/UsersPage.test.tsx > UsersPage > invites a new user
AssertionError: expected "vi.fn()" to be called with arguments: [ { email: 'new@example.com', …(3) } ]
Number of calls: 0
```

### Objectives

Fix the single failing test by replacing `userEvent.type()` with `fireEvent.change()` on the
email input, consistent with the fix already applied to related tests in the same file.

---

## 2. Research Findings

### Root Cause

**jsdom 29.1.1 broke `userEvent.type()` for `<input type="email">` elements.**

When `userEvent.type()` is called on an email input, jsdom 29.1.1 silently fails to dispatch
`change` events. The React `onChange` handler is never called, so the `email` state inside
`InviteModal` stays `''`.

This leaves the "Send Invite" button with `disabled={!email || !!emailError}` evaluating to
`disabled={true}`. A disabled `<button>` does not fire click events. `inviteMutation.mutate()` is
never called, so `inviteUser()` is never called.

**Evidence in git history:**

| Commit | Message |
|--------|---------|
| `9129b252` | fix: stabilize invite modal URL preview tests against jsdom event changes — "jsdom 29.1.1 incompatibility with email input events" |
| `4eba7d36` | fix: convert remaining URL preview warning test to fake timer pattern — same issue, same fix |

Both commits fixed tests in the `describe('URL preview')` sub-block of `UsersPage.test.tsx` using
`fireEvent.change()`. The top-level `'invites a new user'` test was missed.

### Why the HTML dump shows no modal

The `waitFor` timeout (1000 ms) expired. The HTML dump snapshot was taken after the timeout;
the modal was still rendering correctly but the mutation had never fired — the dump shows only
the main page layout because `waitFor` terminates and the DOM snapshot is captured at that moment.

### Affected tests

| Test | Line | Status |
|------|------|--------|
| `invites a new user` | 267 | **FAILS** — positive assertion detects 0 calls |
| `hides invite link when backend returns a redacted URL` | 369 | passes trivially — negative assertions pass even when `inviteUser` is never called |

### Component context

- **File**: `frontend/src/pages/UsersPage.tsx`
- `InviteModal` sub-component starts at line 58.
- Email input: `type="email"`, controlled via `value={email}` / `onChange={(e) => { setEmail(e.target.value); validateEmail(e.target.value) }}`
- Submit button: `disabled={!email || !!emailError}` — button is disabled when `email` is empty.
- `inviteMutation.mutationFn` calls `inviteUser(request)` from `../api/users`.

---

## 3. Technical Specification

### Change required

**File**: `frontend/src/pages/__tests__/UsersPage.test.tsx`

`fireEvent` and `act` are already imported on line 1.

#### Test 1: `invites a new user` (line 267)

**Before:**
```ts
await user.type(screen.getByPlaceholderText('user@example.com'), 'new@example.com')
```

**After:**
```ts
fireEvent.change(screen.getByPlaceholderText('user@example.com'), { target: { value: 'new@example.com' } })
```

`fireEvent.change` dispatches the synthetic `change` event that React intercepts, calling the
`onChange` handler and updating `email` state. No fake timers are needed because this test does
not assert on the URL preview debounce (500 ms). After `fireEvent.change`, the button is enabled
and the subsequent `await user.click(...)` calls `inviteMutation.mutate()` → `inviteUser(request)`.

#### Test 2: `hides invite link when backend returns a redacted URL` (line 369)

This test uses the same `userEvent.type()` pattern and is broken for the same reason. Its
negative assertions pass silently even when `inviteUser` is not called, masking the bug. Apply
the same fix for correctness:

**Before:**
```ts
await user.type(screen.getByPlaceholderText('user@example.com'), 'manual@example.com')
```

**After:**
```ts
fireEvent.change(screen.getByPlaceholderText('user@example.com'), { target: { value: 'manual@example.com' } })
```

### No component changes needed

The production `UsersPage.tsx` component is correct. Only the test file requires changes.

---

## 4. Implementation Plan

### Phase 1 — Test fix (single commit)

1. Edit `frontend/src/pages/__tests__/UsersPage.test.tsx`:
   - Line 267: replace `user.type(...)` → `fireEvent.change(...)`
   - Line 369: replace `user.type(...)` → `fireEvent.change(...)`
2. Run locally to confirm both tests pass:
   ```bash
   cd /projects/Charon && npx vitest run src/pages/__tests__/UsersPage.test.tsx
   ```
3. Commit.

---

## 5. Acceptance Criteria

- `UsersPage > invites a new user` passes.
- `UsersPage > hides invite link when backend returns a redacted URL` still passes.
- No other tests in `UsersPage.test.tsx` regress.
- CI Vitest job is green.

---

## 6. Commit Slicing Strategy

**Decision**: single commit, single PR — two-line test-only fix, no production code change, no
dependencies, no review risk.

### Commit 1 (only commit)

| Field | Value |
|-------|-------|
| **Scope** | `test(UsersPage)` |
| **Files** | `frontend/src/pages/__tests__/UsersPage.test.tsx` |
| **Lines changed** | 2 |
| **Dependencies** | None |
| **Validation gate** | `npx vitest run src/pages/__tests__/UsersPage.test.tsx` → all pass |
| **Rollback** | Trivial revert; no production impact |

**Commit message:**
```
test(UsersPage): fix invite user test broken by jsdom 29.1.1 email input change

userEvent.type() silently fails on <input type="email"> in jsdom 29.1.1,
leaving email state empty and the Submit button disabled (0 calls to
inviteUser). Replace with fireEvent.change() consistent with the fix
already applied to URL preview tests in the same file (9129b252, 4eba7d36).

Fixes:
- 'invites a new user' (was failing — 0 calls to inviteUser)
- 'hides invite link when backend returns a redacted URL' (was passing
  trivially due to negative assertions; now correctly exercises the path)
```
