package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inlineScriptRe matches <script> elements that have no src attribute.
var inlineScriptRe = regexp.MustCompile(`(?s)<script(?:\s[^>]*)?>(.*?)</script>`)
var scriptSrcRe = regexp.MustCompile(`(?i)^<script[^>]*\ssrc\s*=`)

// TestBuildCSP_CoversInlineScriptsInIndexHTML guards against the CSP script hash
// drifting from the inline anti-flash theme script in frontend/index.html.
func TestBuildCSP_CoversInlineScriptsInIndexHTML(t *testing.T) {
	t.Parallel()

	csp := buildCSP(DefaultSecurityHeadersConfig())

	// The source file is checked; the Vite build (frontend/dist) preserves inline
	// scripts byte-for-byte, so the dist copy is checked too when present.
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "frontend", "index.html"),
		filepath.Join("..", "..", "..", "..", "frontend", "dist", "index.html"),
	}

	checked := 0
	for i, path := range candidates {
		html, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			require.Truef(t, i > 0, "frontend/index.html must be readable: %v", err)
			continue
		}
		for _, m := range inlineScriptRe.FindAllSubmatchIndex(html, -1) {
			openTag := html[m[0]:m[2]]
			if scriptSrcRe.Match(openTag) {
				continue
			}
			sum := sha256.Sum256(html[m[2]:m[3]])
			token := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
			assert.Containsf(t, csp, token,
				"inline <script> in %s is not allowed by the CSP; update the hash in buildCSP", path)
			checked++
		}
	}
	require.NotZero(t, checked, "expected at least one inline script in frontend/index.html")
}
