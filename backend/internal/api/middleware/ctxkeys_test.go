package middleware

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallerAccessors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("empty context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		_, ok := CallerID(c)
		assert.False(t, ok)
		assert.Equal(t, "", CallerRole(c))
		assert.False(t, HasCallerRole(c))
	})

	t.Run("recorded identity", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		SetCaller(c, 7, "admin")
		id, ok := CallerID(c)
		assert.True(t, ok)
		assert.Equal(t, uint(7), id)
		assert.Equal(t, "admin", CallerRole(c))
		assert.True(t, HasCallerRole(c))
	})

	t.Run("unexpected value types are ignored", func(t *testing.T) {
		c, _ := gin.CreateTestContext(nil)
		c.Set(UserIDKey, "7")
		c.Set(RoleKey, 1)
		_, ok := CallerID(c)
		assert.False(t, ok)
		assert.Equal(t, "", CallerRole(c))
	})
}

var identityKeys = map[string]bool{"userID": true, "user_id": true, "role": true}

// identityAccessMethods are the context read/write methods that take a key.
var identityAccessMethods = map[string]bool{
	"Get": true, "GetString": true, "GetUint": true, "GetInt": true,
	"MustGet": true, "Set": true, "Value": true,
}

// collectStringConsts records every string-valued const/var declaration in the
// file by name, so aliases of the identity keys can be resolved.
func collectStringConsts(file *ast.File, out map[string]ast.Expr) {
	ast.Inspect(file, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range vs.Names {
			if i < len(vs.Values) {
				out[name.Name] = vs.Values[i]
			}
		}
		return true
	})
}

// resolveKey returns the string value an expression denotes when it is a
// literal, or an identifier chain leading to one. The exported key constants
// are reported as their own marker.
func resolveKey(e ast.Expr, consts map[string]ast.Expr, depth int) (string, bool) {
	if depth > 8 {
		return "", false
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			v, err := strconv.Unquote(x.Value)
			return v, err == nil
		}
	case *ast.ParenExpr:
		return resolveKey(x.X, consts, depth+1)
	case *ast.CallExpr: // string(...) conversions
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "string" && len(x.Args) == 1 {
			return resolveKey(x.Args[0], consts, depth+1)
		}
	case *ast.Ident:
		if v, ok := consts[x.Name]; ok {
			return resolveKey(v, consts, depth+1)
		}
	case *ast.SelectorExpr:
		if v, ok := consts[x.Sel.Name]; ok {
			return resolveKey(v, consts, depth+1)
		}
	}
	return "", false
}

func isKeyConstRef(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name == "UserIDKey" || x.Name == "RoleKey"
	case *ast.SelectorExpr:
		return x.Sel.Name == "UserIDKey" || x.Sel.Name == "RoleKey"
	}
	return false
}

// identityViolations reports every way a file reads or writes the caller
// identity other than through the accessors in ctxkeys.go.
func identityViolations(fset *token.FileSet, file *ast.File, consts map[string]ast.Expr) []string {
	var out []string
	report := func(n ast.Node, msg string) {
		out = append(out, fset.Position(n.Pos()).String()+": "+msg)
	}
	isIdentity := func(e ast.Expr) bool {
		v, ok := resolveKey(e, consts, 0)
		return ok && identityKeys[v]
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if ok && identityAccessMethods[sel.Sel.Name] && len(x.Args) > 0 && isIdentity(x.Args[0]) {
				report(x, "raw identity key; use the accessors in ctxkeys.go")
			}
		case *ast.IndexExpr:
			if sel, ok := x.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Keys" && isIdentity(x.Index) {
				report(x, "raw identity key in Keys map; use the accessors in ctxkeys.go")
			}
		case *ast.TypeAssertExpr:
			if id, ok := x.Type.(*ast.Ident); ok && id.Name == "uint" {
				report(x, "type assertion to uint; use middleware.CallerID")
			}
		case *ast.Ident:
			if x.Name == "UserIDKey" || x.Name == "RoleKey" {
				report(x, "identity key constant used outside ctxkeys.go")
			}
		case *ast.SelectorExpr:
			if isKeyConstRef(x) && x.Sel.Name != "" {
				report(x, "identity key constant used outside ctxkeys.go")
			}
		}
		return true
	})
	return out
}

func parseSnippet(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "snippet.go", "package p\n"+src, 0)
	require.NoError(t, err)
	return fset, file
}

func TestIdentityGuardFlagsEveryAccessShape(t *testing.T) {
	flagged := map[string]string{
		"literal Get":          `func f(c *gin.Context) { c.Get("userID") }`,
		"literal GetString":    `func f(c *gin.Context) { _ = c.GetString("user_id") }`,
		"literal role":         `func f(c *gin.Context) { _ = c.GetString("role") }`,
		"literal Set":          `func f(c *gin.Context) { c.Set("role", "admin") }`,
		"MustGet":              `func f(c *gin.Context) { _ = c.MustGet("userID") }`,
		"GetUint":              `func f(c *gin.Context) { _ = c.GetUint("userID") }`,
		"Value read":           `func f(c *gin.Context) { _ = c.Request.Context().Value("user_id") }`,
		"aliased const":        `const k = "userID"; func f(c *gin.Context) { c.Get(k) }`,
		"chained alias":        `const a = "role"; const b = a; func f(c *gin.Context) { c.Get(b) }`,
		"aliased var":          `var k = "user_id"; func f(c *gin.Context) { c.Get(k) }`,
		"typed const":          `const k string = "role"; func f(c *gin.Context) { c.Get(k) }`,
		"string conversion":    `func f(c *gin.Context) { c.Get(string("role")) }`,
		"Keys index":           `func f(c *gin.Context) { _ = c.Keys["userID"] }`,
		"Keys index via const": `const k = "role"; func f(c *gin.Context) { _ = c.Keys[k] }`,
		"exported const":       `func f(c *gin.Context) { c.Get(middleware.UserIDKey) }`,
		"bare exported const":  `func f(c *gin.Context) { c.Set(RoleKey, "x") }`,
		"uint assertion":       `func f(v any) { _ = v.(uint) }`,
	}
	for name, src := range flagged {
		t.Run("flags "+name, func(t *testing.T) {
			fset, file := parseSnippet(t, src)
			consts := map[string]ast.Expr{}
			collectStringConsts(file, consts)
			assert.NotEmpty(t, identityViolations(fset, file, consts))
		})
	}

	allowed := map[string]string{
		"accessor use":       `func f(c *gin.Context) { _, _ = middleware.CallerID(c); _ = middleware.CallerRole(c) }`,
		"unrelated key":      `func f(c *gin.Context) { c.Get("emergency_bypass") }`,
		"json map key":       `func f() { m := map[string]any{"role": 1, "user_id": 2}; _ = m["role"] }`,
		"gin.H literal":      `func f() { _ = gin.H{"user_id": 1, "role": "x"} }`,
		"query param":        `func f(c *gin.Context) { _ = c.Query("role") }`,
		"other keys index":   `func f(c *gin.Context) { _ = c.Keys["other"] }`,
		"non-uint assertion": `func f(v any) { _, _ = v.(string) }`,
	}
	for name, src := range allowed {
		t.Run("allows "+name, func(t *testing.T) {
			fset, file := parseSnippet(t, src)
			consts := map[string]ast.Expr{}
			collectStringConsts(file, consts)
			assert.Empty(t, identityViolations(fset, file, consts))
		})
	}
}

func importsGin(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"github.com/gin-gonic/gin"` {
			return true
		}
	}
	return false
}

// TestCallerIdentityAccessedThroughAccessors keeps request identity access in
// one place: outside ctxkeys.go, no non-test code under internal/ or cmd/ that
// handles gin contexts may
// use raw context keys (literal or aliased), the Keys map, the exported key
// constants, or unchecked unsigned-integer assertions.
func TestCallerIdentityAccessedThroughAccessors(t *testing.T) {
	roots := []string{"../../../internal", "../../../cmd"}
	fset := token.NewFileSet()
	var files []*ast.File
	consts := map[string]ast.Expr{}

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			if name == "ctxkeys.go" && filepath.Base(filepath.Dir(path)) == "middleware" {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			require.NoError(t, perr, path)
			if !importsGin(file) {
				return nil
			}
			collectStringConsts(file, consts)
			files = append(files, file)
			return nil
		})
		require.NoError(t, err)
	}
	require.Greater(t, len(files), 50, "guard scanned too few files; check roots")

	for _, file := range files {
		for _, v := range identityViolations(fset, file, consts) {
			t.Error(v)
		}
	}
}
