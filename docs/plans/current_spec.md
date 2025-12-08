# Current Spec & Implementation Plan

## Goal
Clean up Security/CrowdSec frontend test warnings and ensure backend tests run via VS Code task.

## Steps
1) Frontend tests: Update `frontend/src/pages/__tests__/Security*.tsx` to wrap React state updates in `act(...)` and ensure React Query mocks return defined data shapes (provide default objects/arrays instead of `undefined`).
2) Mock defaults: Where tests mock query responses for Security/CrowdSec pages, supply minimal valid data (e.g., empty arrays/objects) to avoid undefined access warnings.
3) Backend tests: Run task "Go: Test Backend" (`shell: Go: Test Backend`) to execute `cd backend && go test ./... -v`.
