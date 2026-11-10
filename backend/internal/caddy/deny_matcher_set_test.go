package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDenyMatcherSetMergesExistingNegation(t *testing.T) {
	positive := map[string]any{
		"path": []string{"/admin"},
		"not":  []map[string]any{{"path": []string{"/public"}}},
	}
	negated := []map[string]any{{"host": []string{"app.example.com"}}}

	set := denyMatcherSet(positive, negated, []string{"192.0.2.1"})

	assert.Equal(t, []string{"/admin"}, set["path"], "positive matchers are kept")
	assert.Equal(t, []map[string]any{
		{"remote_ip": map[string]any{"ranges": []string{"192.0.2.1"}}},
		{"path": []string{"/public"}},
		{"host": []string{"app.example.com"}},
	}, set["not"], "admin ranges, then the existing negation, then the negated sets")
}
