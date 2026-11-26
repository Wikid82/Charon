package upstreamdns

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupTimeoutFromResolvConf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	tests := map[string]struct {
		path string
		want time.Duration
	}{
		"missing file uses the default":      {filepath.Join(dir, "absent"), 3 * time.Second},
		"no options line uses the default":   {write("none", "nameserver 10.0.0.2\n"), 3 * time.Second},
		"timeout two seconds":                {write("two", "options ndots:2 timeout:2\n"), 1500 * time.Millisecond},
		"timeout one second clamps to floor": {write("one", "options timeout:1\n"), 500 * time.Millisecond},
		"timeout ten seconds clamps to cap":  {write("ten", "options timeout:10\n"), 3 * time.Second},
		"unparsable timeout uses default":    {write("bad", "options timeout:x\n"), 3 * time.Second},
		"negative timeout uses default":      {write("neg", "options timeout:-1\n"), 3 * time.Second},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, LookupTimeoutFromResolvConf(tc.path))
		})
	}
}

func TestParseResolvTimeout_SkipsOtherOptions(t *testing.T) {
	t.Parallel()
	got, ok := parseResolvTimeout([]byte("search lan\n\noptions attempts:2 timeout:4\n"))
	assert.True(t, ok)
	assert.Equal(t, 4*time.Second, got)
	_, ok = parseResolvTimeout([]byte("options attempts:2\n"))
	assert.False(t, ok)
}
