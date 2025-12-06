// Test-only type shims to satisfy strict type-checking in CI
// Properly type the default export from @testing-library/user-event
declare module '@testing-library/user-event' {
  import type { UserEvent } from '@testing-library/user-event/dist/types/setup/setup'
  const userEvent: UserEvent
  export default userEvent
  export { userEvent }
  export type { UserEvent }
}
