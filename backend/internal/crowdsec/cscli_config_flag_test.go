package crowdsec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hub items are verified against the engine configuration, so cscli gets `-c <file>` like the other calls.
func TestVerifyHubItemPassesTheConfigFileToCSCLI(t *testing.T) {
	t.Parallel()
	exec := &verifyExec{out: `{"installed":true,"tainted":false}`}

	require.NoError(t, VerifyHubItem(t.Context(), exec, "/data/config/config.yaml", "collections", "crowdsecurity/sshd"))

	i := slices.Index(exec.args, "-c")
	require.GreaterOrEqual(t, i, 0, "cscli was started without -c: %v", exec.args)
	require.Greater(t, len(exec.args), i+1, "-c has no file: %v", exec.args)
	assert.Equal(t, "/data/config/config.yaml", exec.args[i+1])
}

// cscliProbesWithoutConfig lists the functions that start cscli without -c on purpose.
//   - hasCSCLI only probes whether the cscli binary can run (`cscli version`). It reads no configuration, and a
//     broken or missing engine configuration must not make it report cscli as absent.
var cscliProbesWithoutConfig = map[string]bool{"hasCSCLI": true}

// Every function of this package that starts cscli passes the configuration file with -c. The calls are found by
// their literal "cscli" argument, so the check needs no change when a new call is added.
func TestCrowdSecPackageRunsCSCLIOnlyWithTheConfigFile(t *testing.T) {
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
			if !ok || fn.Body == nil {
				continue
			}
			if callsCSCLI(fn.Body) && !cscliProbesWithoutConfig[fn.Name.Name] && !hasStringLiteral(fn.Body, `"-c"`) {
				offenders = append(offenders, name+": "+fn.Name.Name)
			}
		}
	}
	assert.Empty(t, offenders, "these functions start cscli without -c")
}

// stripConfigFlag drops the leading `-c <file>` pair the hub service puts in front of cscli arguments, so
// scripted fakes can match the commands without the configuration path of the test's data directory.
func stripConfigFlag(args []string) []string {
	if len(args) >= 2 && args[0] == "-c" {
		return args[2:]
	}
	return args
}

// callsCSCLI reports whether body calls an Execute method with the literal "cscli" as the command.
func callsCSCLI(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Execute" && sel.Sel.Name != "ExecuteWithEnv") {
			return true
		}
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"cscli"` {
				found = true
			}
		}
		return !found
	})
	return found
}

// hasStringLiteral reports whether body contains the string literal lit (quotes included).
func hasStringLiteral(body *ast.BlockStmt, lit string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if b, ok := n.(*ast.BasicLit); ok && b.Kind == token.STRING && b.Value == lit {
			found = true
		}
		return !found
	})
	return found
}
