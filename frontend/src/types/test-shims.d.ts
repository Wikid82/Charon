// Test-only type shims to satisfy strict type-checking in CI
declare module '@testing-library/user-event' {
  const userEvent: any;
  export default userEvent;
}
