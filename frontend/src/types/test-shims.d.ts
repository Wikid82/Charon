// Test-only type shims to satisfy strict type-checking in CI

// Import jest-dom matchers for vitest
/// <reference types="@testing-library/jest-dom/vitest" />

// Properly type the default export from @testing-library/user-event
declare module '@testing-library/user-event' {
  import type { UserEvent } from '@testing-library/user-event/dist/types/setup/setup'
  const userEvent: UserEvent
  export default userEvent
  export { userEvent }
  export type { UserEvent }
}
