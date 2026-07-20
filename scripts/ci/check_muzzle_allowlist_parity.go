// Command check_muzzle_allowlist_parity is a structural drift guard for GH
// #1161: it fails if the Docker API allowlist declarations in
// backend/internal/orthrus/muzzle.go and agent/muzzle/muzzle.go diverge.
//
// The two files intentionally duplicate their allowlist policy rather than
// sharing an importable package (agent/ is a separate, minimal Go module
// that does not import backend/ packages -- see the "Design decision"
// section of docs/plans/current_spec.md for the full rationale). The
// existing shared behavioral corpus
// (backend/internal/orthrus/testdata/muzzle_corpus.json) proves outcome
// parity for the specific inputs it contains, but cannot catch "a new
// pattern was added to one file's declaration and nobody thought to add a
// corresponding corpus case" -- precisely what caused the production
// incident this checker exists to prevent (commits 98a68b67, b71cbd62,
// eabf358d). This tool closes that gap by structurally comparing the
// *declared* allowlist contents themselves, independent of any specific
// test input.
//
// Usage: go run scripts/ci/check_muzzle_allowlist_parity.go
// Exits 0 if all eight paired declarations match; exits 1 and prints every
// mismatch (not just the first) otherwise.
//
// Scope, and its explicit limitation: this checker diffs eight *data*
// declarations -- the version-prefix regex source and seven
// allowlist/body-size-constant declarations. It deliberately does NOT
// attempt to diff the *logic* of validateNetworkModeValue,
// validateMountsValue, or validateContainerCreateBody -- AST-diffing
// function bodies for semantic (not just textual) equivalence is a much
// harder problem than diffing data-literal declarations. Those functions
// remain covered by the shared behavioral corpus and by each file's doc
// comments cross-referencing the other. This is an accepted, documented
// limitation, not a silent gap.
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	backendMuzzlePath = "backend/internal/orthrus/muzzle.go"
	agentMuzzlePath   = "agent/muzzle/muzzle.go"
)

// httpMethodConstants resolves the small, fixed set of net/http method
// constants used as struct-literal field values in allowedWritePatterns
// (e.g. http.MethodPost) to their string value, without needing a full
// go/types-based type-checker (which would require resolving the net/http
// package's export data). Both muzzle.go files only ever use these
// constants for this purpose, so a fixed lookup table is sufficient and
// keeps this tool free of any dependency beyond the Go standard library.
var httpMethodConstants = map[string]string{
	"MethodGet":     "GET",
	"MethodHead":    "HEAD",
	"MethodPost":    "POST",
	"MethodPut":     "PUT",
	"MethodPatch":   "PATCH",
	"MethodDelete":  "DELETE",
	"MethodConnect": "CONNECT",
	"MethodOptions": "OPTIONS",
	"MethodTrace":   "TRACE",
}

// declKind identifies the AST shape a target declaration's value takes, so
// extractDecl knows which extraction routine to apply.
type declKind int

const (
	kindRegexCall  declKind = iota // versionPrefixRe: regexp.MustCompile("...")
	kindStringSet                  // map[string]struct{}{"a": {}, ...}
	kindStringList                 // []string{"a", "b", ...}
	kindPairList                   // []struct{ prefix, suffix string }{...} or []struct{ method, pattern string }{...}
	kindIntConst                   // a constant integer expression
)

// target describes one of the eight declarations this checker compares.
type target struct {
	name string
	kind declKind
	// fieldA/fieldB name the two struct fields for kindPairList
	// declarations (e.g. "prefix"/"suffix" or "method"/"pattern"); unused
	// for other kinds.
	fieldA, fieldB string
}

var targets = []target{
	{name: "versionPrefixRe", kind: kindRegexCall},
	{name: "allowedDockerPaths", kind: kindStringSet},
	{name: "allowedDockerPatterns", kind: kindStringList},
	{name: "allowedDockerPrefixSuffixPatterns", kind: kindPairList, fieldA: "prefix", fieldB: "suffix"},
	{name: "allowedWriteExactPaths", kind: kindStringSet},
	{name: "allowedWritePatterns", kind: kindPairList, fieldA: "method", fieldB: "pattern"},
	{name: "hostConfigAllowedKeys", kind: kindStringSet},
	{name: "maxContainerCreateBodyBytes", kind: kindIntConst},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "muzzle allowlist parity check FAILED:")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("muzzle allowlist parity check passed: all 8 paired declarations match.")
}

func run() error {
	backendFile, err := parseFile(backendMuzzlePath)
	if err != nil {
		return err
	}
	agentFile, err := parseFile(agentMuzzlePath)
	if err != nil {
		return err
	}

	var problems []string
	for _, tgt := range targets {
		backendExpr, ok := findDeclValue(backendFile, tgt.name)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: declaration not found in %s", tgt.name, backendMuzzlePath))
			continue
		}
		agentExpr, ok := findDeclValue(agentFile, tgt.name)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: declaration not found in %s", tgt.name, agentMuzzlePath))
			continue
		}

		mismatch, err := compareDecl(tgt, backendExpr, agentExpr)
		if err != nil {
			// A declaration that can't be extracted (unexpected shape,
			// e.g. renamed field or changed literal form) is reported
			// explicitly rather than silently skipped.
			problems = append(problems, fmt.Sprintf("%s: %v", tgt.name, err))
			continue
		}
		if mismatch != "" {
			problems = append(problems, mismatch)
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func parseFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

// findDeclValue returns the right-hand-side expression of the top-level
// var/const declaration named name in f, e.g. for
// `var allowedDockerPaths = map[string]struct{}{...}` it returns the
// map[string]struct{}{...} composite literal expression.
func findDeclValue(f *ast.File, name string) (ast.Expr, bool) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range vs.Names {
				if n.Name != name {
					continue
				}
				if i >= len(vs.Values) {
					return nil, false
				}
				return vs.Values[i], true
			}
		}
	}
	return nil, false
}

// compareDecl extracts and diffs one declaration pair, returning a
// human-readable mismatch description (empty string if they match).
func compareDecl(tgt target, backendExpr, agentExpr ast.Expr) (string, error) {
	switch tgt.kind {
	case kindRegexCall:
		backendVal, err := extractRegexSource(backendExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", backendMuzzlePath, tgt.name, err)
		}
		agentVal, err := extractRegexSource(agentExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", agentMuzzlePath, tgt.name, err)
		}
		if backendVal != agentVal {
			return fmt.Sprintf("%s: backend=%s agent=%s (source strings differ)", tgt.name, backendVal, agentVal), nil
		}
		return "", nil

	case kindIntConst:
		backendVal, err := evalConstInt(backendExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", backendMuzzlePath, tgt.name, err)
		}
		agentVal, err := evalConstInt(agentExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", agentMuzzlePath, tgt.name, err)
		}
		if backendVal != agentVal {
			return fmt.Sprintf("%s: backend=%d agent=%d", tgt.name, backendVal, agentVal), nil
		}
		return "", nil

	case kindStringSet:
		backendVals, err := extractStringSet(backendExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", backendMuzzlePath, tgt.name, err)
		}
		agentVals, err := extractStringSet(agentExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", agentMuzzlePath, tgt.name, err)
		}
		return diffStringSets(tgt.name, backendVals, agentVals), nil

	case kindStringList:
		backendVals, err := extractStringList(backendExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", backendMuzzlePath, tgt.name, err)
		}
		agentVals, err := extractStringList(agentExpr)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", agentMuzzlePath, tgt.name, err)
		}
		return diffStringSets(tgt.name, backendVals, agentVals), nil

	case kindPairList:
		backendVals, err := extractPairList(backendExpr, tgt.fieldA, tgt.fieldB)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", backendMuzzlePath, tgt.name, err)
		}
		agentVals, err := extractPairList(agentExpr, tgt.fieldA, tgt.fieldB)
		if err != nil {
			return "", fmt.Errorf("%s (%s): %w", agentMuzzlePath, tgt.name, err)
		}
		return diffStringSets(tgt.name, backendVals, agentVals), nil
	}

	return "", fmt.Errorf("internal error: unhandled decl kind for %s", tgt.name)
}

// extractRegexSource unwraps a `regexp.MustCompile("...")` call expression
// and returns the unquoted string literal passed to it.
func extractRegexSource(e ast.Expr) (string, error) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return "", fmt.Errorf("expected regexp.MustCompile(...) call expression, got %T", e)
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "MustCompile" {
		return "", fmt.Errorf("expected a call to MustCompile, got %s", exprDescription(call.Fun))
	}
	if len(call.Args) != 1 {
		return "", fmt.Errorf("expected exactly one argument to MustCompile, got %d", len(call.Args))
	}
	return stringLiteralValue(call.Args[0])
}

// evalConstInt evaluates a constant integer expression (literals combined
// with +, -, *, /, parens, and unary minus -- sufficient for the one
// int-constant declaration this tool compares,
// `const maxContainerCreateBodyBytes = 64 * 1024`) using go/constant, the
// same arbitrary-precision constant-arithmetic package the Go compiler
// itself uses.
func evalConstInt(e ast.Expr) (int64, error) {
	val, err := evalConstValue(e)
	if err != nil {
		return 0, err
	}
	i, ok := constant.Int64Val(val)
	if !ok {
		return 0, fmt.Errorf("constant expression %s is not representable as an int64", exprDescription(e))
	}
	return i, nil
}

func evalConstValue(e ast.Expr) (constant.Value, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		val := constant.MakeFromLiteral(v.Value, v.Kind, 0)
		if val.Kind() == constant.Unknown {
			return nil, fmt.Errorf("could not parse literal %q", v.Value)
		}
		return val, nil
	case *ast.ParenExpr:
		return evalConstValue(v.X)
	case *ast.UnaryExpr:
		x, err := evalConstValue(v.X)
		if err != nil {
			return nil, err
		}
		return constant.UnaryOp(v.Op, x, 0), nil
	case *ast.BinaryExpr:
		x, err := evalConstValue(v.X)
		if err != nil {
			return nil, err
		}
		y, err := evalConstValue(v.Y)
		if err != nil {
			return nil, err
		}
		return constant.BinaryOp(x, v.Op, y), nil
	default:
		return nil, fmt.Errorf("expected a constant integer expression, got %T (%s)", e, exprDescription(e))
	}
}

// extractStringSet extracts the string keys of a `map[string]struct{}{...}`
// composite literal.
func extractStringSet(e ast.Expr) ([]string, error) {
	comp, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("expected a map composite literal, got %T", e)
	}
	if _, ok := comp.Type.(*ast.MapType); !ok {
		return nil, fmt.Errorf("expected a map[string]struct{} composite literal, got type %s", exprDescription(comp.Type))
	}
	var out []string
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, fmt.Errorf("expected key: value map entries, got %T", elt)
		}
		s, err := stringLiteralValue(kv.Key)
		if err != nil {
			return nil, fmt.Errorf("map key: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}

// extractStringList extracts the string elements of a `[]string{...}`
// composite literal.
func extractStringList(e ast.Expr) ([]string, error) {
	comp, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("expected a slice composite literal, got %T", e)
	}
	var out []string
	for _, elt := range comp.Elts {
		s, err := stringLiteralValue(elt)
		if err != nil {
			return nil, fmt.Errorf("slice element: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}

// extractPairList extracts a `[]struct{ fieldA, fieldB string }{{fieldA:
// ..., fieldB: ...}, ...}` composite literal into "valueA valueB" strings,
// one per element, resolving net/http method-constant selector expressions
// (e.g. http.MethodPost) via httpMethodConstants as well as plain string
// literals.
func extractPairList(e ast.Expr, fieldA, fieldB string) ([]string, error) {
	comp, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("expected a slice composite literal, got %T", e)
	}
	var out []string
	for _, elt := range comp.Elts {
		structLit, isCompositeLit := elt.(*ast.CompositeLit)
		if !isCompositeLit {
			return nil, fmt.Errorf("expected a struct composite literal element, got %T", elt)
		}
		fields := map[string]string{}
		for _, fieldElt := range structLit.Elts {
			kv, isKeyValue := fieldElt.(*ast.KeyValueExpr)
			if !isKeyValue {
				return nil, fmt.Errorf("expected keyed struct fields (e.g. %s: ...), got %T", fieldA, fieldElt)
			}
			key, isIdent := kv.Key.(*ast.Ident)
			if !isIdent {
				return nil, fmt.Errorf("expected an identifier struct field key, got %T", kv.Key)
			}
			val, err := stringLiteralValue(kv.Value)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", key.Name, err)
			}
			fields[key.Name] = val
		}
		a, hasFieldA := fields[fieldA]
		if !hasFieldA {
			return nil, fmt.Errorf("struct element missing field %q", fieldA)
		}
		b, hasFieldB := fields[fieldB]
		if !hasFieldB {
			return nil, fmt.Errorf("struct element missing field %q", fieldB)
		}
		out = append(out, a+" "+b)
	}
	return out, nil
}

// stringLiteralValue resolves e to a string value, handling both plain
// string literals and the fixed set of net/http method-constant selector
// expressions used in allowedWritePatterns.
func stringLiteralValue(e ast.Expr) (string, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", fmt.Errorf("expected a string literal, got a %s literal", v.Kind)
		}
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", fmt.Errorf("unquote %q: %w", v.Value, err)
		}
		return s, nil
	case *ast.SelectorExpr:
		pkgIdent, ok := v.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "http" {
			return "", fmt.Errorf("unsupported selector expression %s", exprDescription(e))
		}
		val, ok := httpMethodConstants[v.Sel.Name]
		if !ok {
			return "", fmt.Errorf("unrecognized net/http method constant http.%s", v.Sel.Name)
		}
		return val, nil
	default:
		return "", fmt.Errorf("expected a string literal or http.MethodXxx constant, got %T", e)
	}
}

// exprDescription renders a short, best-effort description of an AST node
// for error messages, without pulling in go/printer + a token.FileSet at
// every call site.
func exprDescription(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprDescription(v.X) + "." + v.Sel.Name
	case *ast.MapType:
		return "map[" + exprDescription(v.Key) + "]" + exprDescription(v.Value)
	case *ast.ArrayType:
		return "[]" + exprDescription(v.Elt)
	case *ast.StructType:
		return "struct{...}"
	default:
		return fmt.Sprintf("%T", e)
	}
}

// diffStringSets compares two sets of extracted strings (allowing
// duplicates to collapse, since both are semantically sets), returning a
// human-readable mismatch description, or an empty string if they're
// identical.
func diffStringSets(declName string, backendVals, agentVals []string) string {
	backendSet := toSet(backendVals)
	agentSet := toSet(agentVals)

	var missingInAgent, missingInBackend []string
	for v := range backendSet {
		if !agentSet[v] {
			missingInAgent = append(missingInAgent, v)
		}
	}
	for v := range agentSet {
		if !backendSet[v] {
			missingInBackend = append(missingInBackend, v)
		}
	}
	sort.Strings(missingInAgent)
	sort.Strings(missingInBackend)

	if len(missingInAgent) == 0 && len(missingInBackend) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s:", declName)
	if len(missingInAgent) > 0 {
		fmt.Fprintf(&b, " present in backend, missing in agent: {%s}", strings.Join(missingInAgent, ", "))
	}
	if len(missingInBackend) > 0 {
		fmt.Fprintf(&b, " present in agent, missing in backend: {%s}", strings.Join(missingInBackend, ", "))
	}
	return b.String()
}

func toSet(vals []string) map[string]bool {
	out := make(map[string]bool, len(vals))
	for _, v := range vals {
		out[v] = true
	}
	return out
}
