package routes

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// passwordCheckAllowlist lists every function allowed to verify an account
// password, and whether it must call the sign-in throttle guard first. Login
// and ChangePassword are throttled by the /api/v1/auth route group instead.
var passwordCheckAllowlist = map[string]bool{
	"AuthService.Login":          false,
	"AuthService.ChangePassword": false,
	"UserHandler.UpdateProfile":  true,
	"CertificateHandler.Export":  true,
}

// funcDeclName returns "Recv.Name" for methods and "Name" for functions.
func funcDeclName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if ident, ok := typ.(*ast.Ident); ok {
		return ident.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// passwordCheckSite records every CheckPassword selector and every throttle
// guard call found in one function body.
type passwordCheckSite struct {
	file       string
	checkPos   []token.Pos
	checkLines []int
	guardPos   []token.Pos
}

// TestPasswordVerificationCallSitesAreGuarded fails when a new account-password
// check appears outside the allowlist, or when an allowlisted handler that
// needs the sign-in throttle guard has a CheckPassword call that is not
// preceded (by source position) by its own guard call: the i-th check needs at
// least i guard calls before it, so a second unguarded check is caught even
// when the first one is guarded. Route tests cover the runtime behavior.
func TestPasswordVerificationCallSitesAreGuarded(t *testing.T) {
	backendRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)

	fset := token.NewFileSet()
	sites := map[string][]passwordCheckSite{}
	for _, dir := range []string{"internal", "cmd"} {
		walkErr := filepath.WalkDir(filepath.Join(backendRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if parseErr != nil {
				return parseErr
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				site := passwordCheckSite{file: path}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.SelectorExpr:
						switch x.Sel.Name {
						case "CheckPassword":
							site.checkPos = append(site.checkPos, x.Pos())
							site.checkLines = append(site.checkLines, fset.Position(x.Pos()).Line)
						case "AllowPasswordAttempt":
							site.guardPos = append(site.guardPos, x.Pos())
						}
					case *ast.CallExpr:
						if ident, ok := x.Fun.(*ast.Ident); ok && ident.Name == "allowPasswordAttempt" {
							site.guardPos = append(site.guardPos, x.Pos())
						}
					}
					return true
				})
				if len(site.checkPos) > 0 {
					name := funcDeclName(fn)
					sites[name] = append(sites[name], site)
				}
			}
			return nil
		})
		require.NoError(t, walkErr)
	}

	for name, found := range sites {
		needsGuard, allowed := passwordCheckAllowlist[name]
		if !assert.True(t, allowed, "%s verifies an account password (%s:%d); guard it with the sign-in throttle and add it to passwordCheckAllowlist",
			name, found[0].file, found[0].checkLines[0]) {
			continue
		}
		if !needsGuard {
			continue
		}
		for _, s := range found {
			for i, checkPos := range s.checkPos {
				guards := 0
				for _, g := range s.guardPos {
					if g < checkPos {
						guards++
					}
				}
				assert.GreaterOrEqual(t, guards, i+1,
					"%s in %s must call the password attempt guard before CheckPassword (line %d)", name, s.file, s.checkLines[i])
			}
		}
	}
	for name := range passwordCheckAllowlist {
		assert.Contains(t, sites, name, "allowlisted password check %s was not found; update the allowlist", name)
	}
}
