package version

import (
"testing"

"github.com/stretchr/testify/assert"
)

func TestFull(t *testing.T) {
// Default
assert.Contains(t, Full(), Version)

// With build info
originalBuildTime := BuildTime
originalGitCommit := GitCommit
defer func() {
BuildTime = originalBuildTime
GitCommit = originalGitCommit
}()

BuildTime = "2023-01-01"
GitCommit = "abcdef"

full := Full()
assert.Contains(t, full, "2023-01-01")
assert.Contains(t, full, "abcdef")
}
