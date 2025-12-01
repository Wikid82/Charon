// ambient module declaration to satisfy typescript in editor environment
declare module '@testing-library/user-event' {
  const userEvent: any;
  export default userEvent;
}
