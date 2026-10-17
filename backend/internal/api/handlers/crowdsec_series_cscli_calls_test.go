package handlers

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Handlers start cscli through runCSCLI only: it adds the configuration file and the deadline. A handler that
// calls CmdExec.Execute("cscli", ...) itself is listed here.
func TestHandlersRunCSCLIOnlyThroughTheHelper(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var offenders []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(name) //nolint:gosec // G304: Go files of this package
		require.NoError(t, readErr)
		file, parseErr := parser.ParseFile(fset, name, src, 0)
		require.NoError(t, parseErr)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "runCSCLI" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool { //nolint:unparam // ast.Inspect needs the bool
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Execute" {
					return true
				}
				if recv, ok := sel.X.(*ast.SelectorExpr); !ok || recv.Sel.Name != "CmdExec" {
					return true
				}
				if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"cscli"` {
					offenders = append(offenders, name+": "+fn.Name.Name)
				}
				return true
			})
		}
	}
	assert.Empty(t, offenders, "these handlers call cscli without runCSCLI")
}

// The bouncer key check and the bouncer registration pass the engine's configuration file to cscli.
func TestBouncerCalls_PassTheConfigFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o750))
	require.NoError(t, os.WriteFile(cfg, []byte("common:\n  log_level: info\n"), 0o600))

	t.Run("validate the saved key", func(t *testing.T) {
		t.Parallel()
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte(`[]`), nil }}
		h := pr4Handler(t, ex, dir)

		h.validateBouncerKey(context.Background())

		require.NotZero(t, ex.callCount())
		assert.Equal(t, []string{"-c", cfg}, ex.lastCall(t)[:2])
	})

	t.Run("register the bouncer", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cfg := filepath.Join(dir, "config", "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o750))
		require.NoError(t, os.WriteFile(cfg, []byte("common:\n  log_level: info\n"), 0o600))
		ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte(strings.Repeat("k", 32)), nil }}
		h := pr4Handler(t, ex, dir)

		_, _ = h.registerAndSaveBouncer(context.Background())

		require.NotZero(t, ex.callCount())
		ex.mu.Lock()
		defer ex.mu.Unlock()
		for _, call := range ex.calls {
			assert.Equal(t, []string{"-c", cfg}, call[:2], "%v", call)
		}
	})
}

// The bouncer calls keep their own short deadlines through the helper.
func TestBouncerCalls_KeepTheirDeadlines(t *testing.T) {
	t.Parallel()
	ex := &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte(`[]`), nil }}
	h := pr4Handler(t, ex, t.TempDir())

	h.validateBouncerKey(context.Background())

	left := ex.lastRemaining(t)
	assert.Positive(t, left)
	assert.LessOrEqual(t, left, 5*time.Second+time.Second)
}
