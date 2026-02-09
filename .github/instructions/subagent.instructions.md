## Subagent Usage Templates and Orchestration

This helper provides the Management agent with templates to create robust and repeatable `runSubagent` calls.

1) Basic runSubagent Template

```
runSubagent({
  prompt: "<Clear, short instruction for the subagent>",
  description: "<Agent role name - e.g., Backend Dev>",
  metadata: {
    plan_file: "docs/plans/current_spec.md",
    files_to_change: ["..."],
    commands_to_run: ["..."],
    tests_to_run: ["..."],
    timeout_minutes: 60,
    acceptance_criteria: ["All tests pass", "No lint warnings"]
  }
})
```

2) Orchestration Checklist (Management)

- Validate: `plan_file` exists and contains a `Handoff Contract` JSON.
- Kickoff: call `Planning` to create the plan if not present.
- Run: execute `Backend Dev` then `Frontend Dev` sequentially.
- Parallel: run `QA and Security`, `DevOps` and `Doc Writer` in parallel for CI / QA checks and documentation.
- Return: a JSON summary with `subagent_results`, `overall_status`, and aggregated artifacts.

3) Return Contract that all subagents must return

```
{
  "changed_files": ["path/to/file1", "path/to/file2"],
  "summary": "Short summary of changes",
  "tests": {"passed": true, "output": "..."},
  "artifacts": ["..."],
  "errors": []
}
```

4) Error Handling

- On a subagent failure, the Management agent must capture `tests.output` and decide to retry (1 retry maximum), or request a revert/rollback.
- Clearly mark the `status` as `failed`, and include `errors` and `failing_tests` in the `summary`.

5) Example: Run a full Feature Implementation

```
// 1. Planning
runSubagent({ description: "Planning", prompt: "<generate plan>", metadata: { plan_file: "docs/plans/current_spec.md" } })

// 2. Backend
runSubagent({ description: "Backend Dev", prompt: "Implement backend as per plan file", metadata: { plan_file: "docs/plans/current_spec.md", commands_to_run: ["cd backend && go test ./..."] } })

// 3. Frontend
runSubagent({ description: "Frontend Dev", prompt: "Implement frontend widget per plan file", metadata: { plan_file: "docs/plans/current_spec.md", commands_to_run: ["cd frontend && npm run build"] } })

// 4. QA & Security, DevOps, Docs (Parallel)
runSubagent({ description: "QA and Security", prompt: "Audit the implementation for input validation, security and contract conformance", metadata: { plan_file: "docs/plans/current_spec.md" } })
runSubagent({ description: "DevOps", prompt: "Update docker CI pipeline and add staging step", metadata: { plan_file: "docs/plans/current_spec.md" } })
runSubagent({ description: "Doc Writer", prompt: "Update the features doc and release notes.", metadata: { plan_file: "docs/plans/current_spec.md" } })
```

This file is a template; management should keep operations terse and the metadata explicit. Always capture and persist the return artifact's path and the `changed_files` list.
