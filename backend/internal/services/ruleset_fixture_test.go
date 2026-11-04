package services

// testRuleSetContent is a minimal rule set body with one real rule line. A rule set
// without a rule line must not be saved, so fixtures that save a rule set use this.
const testRuleSetContent = `SecRule ARGS "@contains test" "id:1,phase:1,deny"`
