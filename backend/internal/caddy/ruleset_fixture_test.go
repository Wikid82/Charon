package caddy

// testRuleSetContent is a minimal rule set body with one real rule line. A rule set
// without a rule line is refused on save and skipped on apply, so fixtures that
// expect an enforced rule set must carry at least one rule line.
const testRuleSetContent = `SecRule ARGS "@contains test" "id:1,phase:1,deny"`
