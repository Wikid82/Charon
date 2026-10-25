// Package cscliguard finds functions that start the cscli binary more often than they pass it the
// configuration flag. Test packages share this rule so every package checks cscli starts the same way.
package cscliguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// cscliCommand is the literal command argument that marks a cscli start.
const cscliCommand = "cscli"

// Offenders parses the non-test Go files directly in dir and returns "file: function" for every function
// that starts cscli more often than it calls one of helpers. A start is a call of Execute or ExecuteWithEnv
// with a literal "cscli" argument, or exec.Command / exec.CommandContext with a literal "cscli" command. A nil
// helpers list makes every start an offender. Functions named in exempt are skipped.
//
// It returns an error when dir cannot be read or parsed, or holds no Go files to check, so an empty result
// always means the files were scanned and none offended.
func Offenders(dir string, helpers []string, exempt map[string]bool) ([]string, error) {
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	fset := token.NewFileSet()
	var offenders []string
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path) //nolint:gosec // G304: Go source files of a directory chosen by a test
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		scanned++
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || exempt[fn.Name.Name] {
				continue
			}
			starts := countCSCLIStarts(fn.Body)
			if starts > 0 && countHelperCalls(fn.Body, helpers) < starts {
				offenders = append(offenders, name+": "+fn.Name.Name)
			}
		}
	}
	if scanned == 0 {
		return nil, fmt.Errorf("no Go files to check in %s", dir)
	}
	return offenders, nil
}

// countCSCLIStarts counts the calls in body that start the literal "cscli" command.
func countCSCLIStarts(body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && startsCSCLI(call) {
			count++
		}
		return true
	})
	return count
}

// startsCSCLI reports whether call starts the literal "cscli" command. Execute and ExecuteWithEnv take the
// command anywhere in their arguments; exec.Command takes it first and exec.CommandContext second, after ctx.
func startsCSCLI(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Execute", "ExecuteWithEnv":
		return slices.ContainsFunc(call.Args, isCSCLILiteral)
	case "Command":
		return isExecPackage(sel.X) && len(call.Args) > 0 && isCSCLILiteral(call.Args[0])
	case "CommandContext":
		return isExecPackage(sel.X) && len(call.Args) > 1 && isCSCLILiteral(call.Args[1])
	default:
		return false
	}
}

// isExecPackage reports whether x names the os/exec package. It matches the identifier only, so a local
// variable called exec is treated as the package too.
func isExecPackage(x ast.Expr) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == "exec"
}

// isCSCLILiteral reports whether expr is a string literal equal to "cscli".
func isCSCLILiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	return err == nil && value == cscliCommand
}

// countHelperCalls counts the calls in body to any plain function named in helpers.
func countHelperCalls(body *ast.BlockStmt, helpers []string) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && slices.Contains(helpers, id.Name) {
				count++
			}
		}
		return true
	})
	return count
}
