# Troubleshooting gopls / VS Code Go errors in Charon

This page documents how to triage and collect logs for persistent Go errors shown by gopls or VS Code in the Charon repository.

Steps:

1. Open the Charon workspace in VS Code (project root).
2. Accept the workspace settings prompt to apply .vscode/settings.json.
3. Run the workspace task: Go: Build Backend (or run ./scripts/check_go_build.sh).
4. If errors persist, run ./scripts/gopls_collect.sh and attach the output directory to an issue.

When filing upstream issues, include gopls.log, go-version.txt, go-env.txt, and a short reproduction.
