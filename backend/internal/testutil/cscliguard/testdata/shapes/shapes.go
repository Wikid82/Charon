package shapes

// Each function below starts cscli with a literal "cscli" command argument, except the ones marked otherwise.

func executeStart(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", "version")
}

func executeWithEnvStart(r runner, ctx context.Context) {
	_, _ = r.ExecuteWithEnv(ctx, nil, "cscli", "version")
}

func execCommandStart() {
	_ = exec.Command("cscli", "version")
}

func execCommandContextStart(ctx context.Context) {
	_ = exec.CommandContext(ctx, "cscli", "version")
}

// variableNameStart passes the command in a variable, so it is not a literal start and is not flagged.
func variableNameStart(ctx context.Context, name string) {
	_ = exec.CommandContext(ctx, name, "version")
}

// otherCommandStart runs another binary, so it is not flagged.
func otherCommandStart(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "docker", "ps")
}

func exemptStart(r runner, ctx context.Context) {
	_, _ = r.Execute(ctx, "cscli", "version")
}

func noStart() {}
