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

type passwordCheckSite struct {
	file      string
	checkPos  token.Pos
	guardPos  token.Pos
	checkLine int
}

// TestPasswordVerificationCallSitesAreGuarded fails when a new account-password
// check appears outside the allowlist, or when an allowlisted handler stops
// calling the sign-in throttle guard before (by source position) the check.
// Route tests cover the runtime behavior.
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
						if x.Sel.Name == "CheckPassword" && site.checkPos == token.NoPos {
							site.checkPos = x.Pos()
							site.checkLine = fset.Position(x.Pos()).Line
						}
						if x.Sel.Name == "AllowPasswordAttempt" && site.guardPos == token.NoPos {
							site.guardPos = x.Pos()
						}
					case *ast.CallExpr:
						if ident, ok := x.Fun.(*ast.Ident); ok && ident.Name == "allowPasswordAttempt" && site.guardPos == token.NoPos {
							site.guardPos = x.Pos()
						}
					}
					return true
				})
				if site.checkPos != token.NoPos {
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
			name, found[0].file, found[0].checkLine) {
			continue
		}
		if needsGuard {
			for _, s := range found {
				assert.True(t, s.guardPos != token.NoPos && s.guardPos < s.checkPos,
					"%s must call the password attempt guard before CheckPassword (%s:%d)", name, s.file, s.checkLine)
			}
		}
	}
	for name := range passwordCheckAllowlist {
		assert.Contains(t, sites, name, "allowlisted password check %s was not found; update the allowlist", name)
	}
}
