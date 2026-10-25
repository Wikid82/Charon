package skips

// Test files are not scanned, so this cscli start is not reported.
func testOnlyStart(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", "version")
}
