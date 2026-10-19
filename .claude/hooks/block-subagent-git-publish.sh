#!/usr/bin/env bash
# PreToolUse hook (Bash): subagents may not commit, push, or open/merge PRs.
# Only the main orchestrating session does that (see CLAUDE.md "Orchestration Model").
# Subagent tool calls carry an agent_id in the hook input; the main session's do not.
set -euo pipefail

input=$(cat)
agent_id=$(jq -r '.agent_id // empty' <<<"$input")
[ -z "$agent_id" ] && exit 0

command=$(jq -r '.tool_input.command // empty' <<<"$input")

# Matches the verb anywhere in the command line, tolerating global flags such as `git -C dir commit`.
if grep -Eq '(^|[;&|(`[:space:]])(git([[:space:]]+-[^[:space:]]+([[:space:]]+[^[:space:]-][^[:space:]]*)?)*[[:space:]]+(commit|push)|gh[[:space:]]+pr[[:space:]]+(create|merge))([[:space:]]|$)' <<<"$command"; then
  echo "Blocked: subagents must not commit, push, or open/merge PRs. Leave changes in the working tree and report back to the main session, which owns git publishing (CLAUDE.md)." >&2
  exit 2
fi
