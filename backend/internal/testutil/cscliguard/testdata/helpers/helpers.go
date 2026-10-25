package helpers

// oneStartOneHelper starts cscli once and wraps the arguments once, so it is clean.
func oneStartOneHelper(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", withConfigFlag("version")...)
}

// twoStartsOneHelper starts cscli twice but wraps the arguments once, so it is an offender.
func twoStartsOneHelper(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", withConfigFlag("version")...)
	_, _ = r.Execute(ctx, "cscli", "metrics")
}

// twoStartsTwoHelpers starts cscli twice and wraps the arguments twice, so it is clean.
func twoStartsTwoHelpers(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", withConfigFlag("version")...)
	_, _ = r.Execute(ctx, "cscli", withConfigFlag("metrics")...)
}

// helperOnly wraps arguments without starting cscli, so it is clean.
func helperOnly() []string {
	return withConfigFlag("version")
}
