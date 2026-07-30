# Hotfix: Null Docker Container List Crashes Add Proxy Host Form

**Type**: Standalone two-file hotfix (not a feature). Branch: `fix/docker-empty-list-null-crash` (off `origin/main`).

## 1. Problem Statement

When Charon's Docker integration connects successfully to a Docker daemon that currently has zero running containers, the "Add Proxy Host" page crashes with `Uncaught TypeError: can't access property "map", P is null` at `frontend/src/components/ProxyHostForm.tsx:781`. Root cause is two-sided:

1. **Backend** — `backend/internal/services/docker_service.go`, `ListContainers` (~lines 107-194) declares `var result []DockerContainer` and only `append`s inside the loop. With zero containers, `result` stays a nil Go slice. The handler (`backend/internal/api/handlers/docker_handler.go:124`, `c.JSON(http.StatusOK, containers)`) lets `encoding/json` marshal that nil slice as JSON `null` rather than `[]`.
2. **Frontend** — `frontend/src/hooks/useDocker.ts:7` uses `data: containers = []` in the `useQuery` destructure. That default only fires when `data` is `undefined`; a literal JSON `null` response resolves to `data: null`, so `containers` is `null` at runtime despite its `DockerContainer[]` type, and `ProxyHostForm.tsx` crashes dereferencing it (`.find()` at lines 612 and 672 share the same latent assumption, `.map()` at line 781 is what actually throws today).

Both sides get a defense-in-depth fix — intentional redundancy, not overlap: the frontend contract must not depend on the backend never sending `null`, and the backend response contract should be correct independent of frontend handling.

**Out of scope** (do not touch):
- The separate `isDockerConnectivityError` classification bug tracked at https://github.com/Wikid82/Charon/issues/1205.
- Rootless Docker / subgid host configuration.
- Any other nullable-list pattern elsewhere in the codebase, or general `ProxyHostForm.tsx` refactoring beyond not crashing on null/empty containers.

## 2. Exact Change Spec

### 2.1 Backend — `backend/internal/services/docker_service.go`

Function `ListContainers` (currently ~line 141):

```go
var result []DockerContainer
```

changes to:

```go
result := make([]DockerContainer, 0)
```

No other lines in this function change. This guarantees that when `containers.Items` is empty, `result` is a non-nil, zero-length slice, so `encoding/json` serializes it as `[]` instead of `null` when returned through `docker_handler.go:124`'s `c.JSON(http.StatusOK, containers)`. No handler change is required — the fix is entirely at the source of the slice.

### 2.2 Frontend — `frontend/src/hooks/useDocker.ts`

Current (`useDocker`, lines 6-31):

```ts
const {
  data: containers = [],
  isLoading,
  error,
  refetch,
} = useQuery({ ... })

return {
  containers,
  isLoading,
  error,
  refetch,
}
```

Change: keep the `useQuery` destructure as-is (renaming is not required), but coerce nullishness on the way out of the hook so the returned `containers` can never be `null`/`undefined` regardless of what the API returned:

```ts
return {
  containers: containers ?? [],
  isLoading,
  error,
  refetch,
}
```

This is the only line that changes. It makes the hook's output contract (`DockerContainer[]`, never null) hold even if a future backend regression reintroduces a `null` body, independent of the backend fix in 2.1.

No changes to `ProxyHostForm.tsx` are required or in scope — its `.find()`/`.map()` calls already assume an array, which the hook now guarantees.

## 3. Test Plan

### 3.1 Backend — `backend/internal/services/docker_service_test.go`

Add one new test (place near existing `TestListContainers_ContainerMappingEdgeCases`, reusing the existing `newContainerListClient(t, containerJSON)` helper already defined in this file at line 361):

- **`TestListContainers_EmptyResultIsNotNil`**: call `newContainerListClient(t, "[]")` to stub a Docker daemon response of an empty container array, construct a `DockerService{client: ..., initErr: nil, localHost: "tcp://localhost:2375"}`, call `svc.ListContainers(context.Background(), "")`, and assert:
  - `err` is `nil`
  - `containers` is non-nil (`require.NotNil(t, containers)`)
  - `len(containers) == 0`
  - `json.Marshal(containers)` produces exactly `[]byte("[]")`, not `"null"` (the behavior that actually matters to the frontend contract)

### 3.2 Frontend — `frontend/src/hooks/__tests__/useDocker.test.tsx`

This file already exists with a `describe('useDocker', ...)` block, a `createWrapper()` helper, and a mocked `dockerApi.listContainers`. Add two new `it` cases following the existing pattern (mock `dockerApi.listContainers` to resolve with the value under test, `renderHook(() => useDocker('192.168.1.100'), { wrapper: createWrapper() })`, `waitFor` on `isLoading === false`):

- **`'returns an empty array when the API resolves null'`**: `vi.mocked(dockerApi.listContainers).mockResolvedValue(null as unknown as DockerContainer[])`; assert `result.current.containers` is `[]` (not `null`) and has length 0 — no thrown error.
- **`'returns an empty array when the API resolves an empty array'`**: `vi.mocked(dockerApi.listContainers).mockResolvedValue([])`; assert `result.current.containers` is `[]`.

### 3.3 Frontend — `frontend/src/components/__tests__/ProxyHostForm.test.tsx` (no change — out of scope)

**Decision: do not add a regression test to this file.** Verified against the actual file content (not just the reported description):

- `vi.mock('../../hooks/useDocker', ...)` is declared once at module scope (lines 24-42), and every existing Docker-flow test overrides it by calling `vi.mocked(useDocker).mockReturnValue({...})` directly on the hook's return value (confirmed at lines 465-466, 1419-1420, 1464-1465, 1610-1611). `dockerApi` (`../../api/docker`) is never mocked anywhere in this file — the component tests only ever interact with the hook's mocked output, never the API layer the hook wraps.
- The file's `afterEach` calls `vi.clearAllMocks()` (line 181), not `vi.resetAllMocks()` — mock *implementations* set via `mockReturnValue` persist across tests, they're only cleared of call history. The entire file's Docker-testing convention is built around swapping the hook's return value, not around exercising the hook's internals.
- Consequently, a test that follows this file's established pattern and sets the mocked `useDocker` return value's `containers` to `null` directly would never execute the real `containers ?? []` coercion in `useDocker.ts` — that line only exists inside the hook, and the hook itself is fully replaced by the mock. Such a test would either falsely pass (if it merely asserts on the container list being empty, which is true of the mock data by construction) or reproduce the crash (if `ProxyHostForm.tsx`'s bare `.map()` at line 782 receives the literal `null` — since `ProxyHostForm.tsx` itself has no local null-guard, unlike the `safeDomains = Array.isArray(domains) ? domains : []` guard at line 386 for `useDomains`, and adding one is explicitly out of scope per section 1). Either way, the test would not be exercising the fix.
- Retrofitting real hook execution into this one test (e.g., `vi.doUnmock('../../hooks/useDocker')` plus a fresh dynamic import and a new `vi.mock('../../api/docker', ...)`) would fight the file's existing architecture: it's a single shared 1600+ line spec file with a persistent module-level mock and no per-test module reset, so partially unmocking one hook for one `it()` risks leaking real-hook state or timing into adjacent tests that assume the mock is always active. That is disproportionate rigging for a tight, two-file hotfix.
- Coverage for "the page doesn't crash on null containers" is therefore carried entirely by the hook-level tests in 3.2 (`useDocker.test.tsx`), which is the only place the fix actually lives: those tests call the real `useDocker` (via `renderHook`, with only `dockerApi.listContainers` mocked at the network boundary) and assert `result.current.containers` is coerced to `[]` when the API resolves `null`. Since `ProxyHostForm.tsx` consumes `dockerContainers` exclusively through `useDocker`'s return value, and that return value is now guaranteed to be an array before it ever reaches the component, hook-level coverage is sufficient to satisfy the "no crash" acceptance criterion without touching `ProxyHostForm.test.tsx`.
- No new file is added or modified for component-level regression coverage. `ProxyHostForm.test.tsx` is unchanged by this hotfix.

## 4. Commit Slicing Strategy

Single PR (`fix/docker-empty-list-null-crash` → `main`), two ordered commits — split along the backend/frontend boundary because each side has an independent test suite and validation gate, and splitting keeps each commit's diff reviewable as "one root cause, one fix, one test." Combining into one commit was considered but rejected: the two fixes touch different toolchains (Go vs. Vitest) and reviewers benefit from running `go test` and `npm test` against each commit independently via `git bisect`/checkout.

**Commit 1 — `fix: return empty slice instead of nil from Docker ListContainers`**
- Scope: backend-only, closes the `null` JSON body at its source.
- Files: `backend/internal/services/docker_service.go`, `backend/internal/services/docker_service_test.go`
- Dependency: none (first commit on the branch)
- Validation gate: `cd backend && go test ./internal/services/...` (and `go build ./...` / `make lint-staticcheck-only` per standard backend workflow) — must pass with the new `TestListContainers_EmptyResultIsNotNil` green.

**Commit 2 — `fix: prevent null Docker container list from crashing proxy host form`**
- Scope: frontend-only, makes `useDocker` resilient to nullish API data independent of the backend fix.
- Files: `frontend/src/hooks/useDocker.ts`, `frontend/src/hooks/__tests__/useDocker.test.tsx`
- Dependency: logically independent of Commit 1 (defense-in-depth), but ordered second so the PR reads backend-root-cause-first, frontend-hardening-second.
- Validation gate: `cd frontend && npx vitest run src/hooks/__tests__/useDocker.test.tsx` and `npm run type-check` — must pass with the new cases green. (`ProxyHostForm.test.tsx` is not touched by this PR — see section 3.3 for rationale — but the standard PR-level validation below still runs the full suite, so any incidental regression there would still surface.)

**Commit message convention**: plain `fix:` prefix for both commits — this is not a vulnerability disclosure, so `fix(security):` does not apply.

**PR-level validation** (after both commits, before merge, per repo Definition of Done): `go test ./...`, `npm run build`, `npm run type-check`, relevant Playwright coverage if any existing E2E spec exercises the Add Proxy Host + Docker flow (no new E2E spec is required by this hotfix's scope — this is a unit-test-level regression, reproduced via mocked API responses, not a new user-facing flow).

**Rollback**: both commits are additive, low-risk, single-file-per-commit changes with no schema/migration/API-contract changes (response shape for the non-empty case is unchanged; only the empty case changes from `null` to `[]`, which is what API consumers should already expect from `[]DockerContainer`). If an issue surfaces post-merge, revert Commit 2 and/or Commit 1 independently with plain `git revert` — no data migration or coordinated rollback is needed.

## 5. Acceptance Criteria

- Docker daemon connected with zero containers → GET containers endpoint returns HTTP 200 with JSON body `[]`, not `null`.
- `frontend/src/hooks/useDocker.ts`'s `containers` is always an array (`[]` at minimum), never `null`/`undefined`, regardless of what the API returns.
- "Add Proxy Host" page no longer throws `TypeError: can't access property "map", P is null` when Docker is connected with zero containers.
- New backend test `TestListContainers_EmptyResultIsNotNil` passes.
- New/updated frontend tests in `useDocker.test.tsx` pass (a `ProxyHostForm.test.tsx` regression test was considered and explicitly dropped from scope per section 3.3's rationale — that file mocks `useDocker` wholesale, so a test built on its established pattern would never exercise the real `containers ?? []` fix; hook-level coverage in `useDocker.test.tsx` is the load-bearing test for this fix).
- `go test ./...`, `npm run build`, `npm run type-check` all pass with no regressions to existing tests.
- No changes outside the two production files (`docker_service.go`, `useDocker.ts`) plus their direct test files (`docker_service_test.go`, `useDocker.test.tsx`) — no touching `isDockerConnectivityError`, rootless Docker config, or `ProxyHostForm.tsx` (production or test).
