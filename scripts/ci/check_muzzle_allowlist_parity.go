// Command check_muzzle_allowlist_parity is a structural drift guard for GH
// #1161: it fails if the read-only Docker API allowlist declarations in
// backend/internal/orthrus/muzzle.go and agent/muzzle/muzzle.go diverge.
//
// Write-mode traffic is no longer allowlist-gated on either side (see the
// Muzzle/Filter doc comments in the two muzzle.go files for the full
// rationale: write mode is now a full-access, operator-consent trust
// model), so this checker's remaining scope is the read-only path-matching
// declarations both filters still share and still need to agree on.
//
// The two files intentionally duplicate this policy rather than sharing an
// importable package (agent/ is a separate, minimal Go module that does not
// import backend/ packages). The existing shared behavioral corpus
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
// Exits 0 if all four paired declarations match; exits 1 and prints every
// mismatch (not just the first) otherwise.
package main

import (
	"fmt"
	"go/ast"
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

// declKind identifies the AST shape a target declaration's value takes, so
// extractDecl knows which extraction routine to apply.
type declKind int

const (
	kindRegexCall  declKind = iota // versionPrefixRe: regexp.MustCompile("...")
	kindStringSet                  // map[string]struct{}{"a": {}, ...}
	kindStringList                 // []string{"a", "b", ...}
	kindPairList                   // []struct{ prefix, suffix string }{...}
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
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "muzzle allowlist parity check FAILED:")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("muzzle allowlist parity check passed: all 4 paired declarations match.")
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
// one per element.
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

// stringLiteralValue resolves e to a plain string literal's value.
func stringLiteralValue(e ast.Expr) (string, error) {
	v, ok := e.(*ast.BasicLit)
	if !ok {
		return "", fmt.Errorf("expected a string literal, got %T", e)
	}
	if v.Kind != token.STRING {
		return "", fmt.Errorf("expected a string literal, got a %s literal", v.Kind)
	}
	s, err := strconv.Unquote(v.Value)
	if err != nil {
		return "", fmt.Errorf("unquote %q: %w", v.Value, err)
	}
	return s, nil
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
