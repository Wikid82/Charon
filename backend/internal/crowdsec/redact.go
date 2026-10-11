package crowdsec

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactedPlaceholder replaces every sensitive value of a notification file. It is a plain YAML
// scalar without "$", so CrowdSec's environment expansion leaves it alone.
const RedactedPlaceholder = "<charon-redacted>"

// redactHeaderComment is the only comment a rewritten file carries, and only while it holds at least
// one placeholder.
const redactHeaderComment = "Sensitive values were replaced by Charon when this configuration was exported."

// RedactStubContent is what an exported notification file that cannot be rewritten safely holds: no
// YAML document at all.
const RedactStubContent = "# This file could not be read safely and was left blank by Charon.\n"

const (
	// maxRedactNodes bounds the decoded node count of one file.
	maxRedactNodes = 50_000
	// minCanaryLen is the shortest replaced value the residual scan looks for.
	minCanaryLen = 8
	// maxCanaryWork bounds values x output bytes scanned by the residual check.
	maxCanaryWork = 256 << 20
)

// ErrRedactUnsafe is returned when a notification file cannot be rewritten safely. It carries no
// file content.
var ErrRedactUnsafe = errors.New("notification file cannot be redacted safely")

// RedactResult is the rewritten file.
type RedactResult struct {
	// Data is the redacted file; it never holds an input value classified as sensitive.
	Data []byte
	// Redacted is the number of placeholder scalars in Data (replaced now or already present). Zero
	// means the output holds no placeholder.
	Redacted int
}

// IsPlaceholder reports whether n is the placeholder scalar.
func IsPlaceholder(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!str" && n.Value == RedactedPlaceholder
}

func unsafeRedact(reason string) error {
	return fmt.Errorf("%w: %s", ErrRedactUnsafe, reason)
}

// shape is how a kept key's value may look.
type shape int

const (
	shapeScalar shape = iota
	shapeScalarList
	shapeHeaders
	shapeRotate
)

// keepSet is the allowlist of one mapping. Every key outside it has its whole value replaced.
type keepSet struct {
	keys map[string]shape
	// headers keeps every key (header names) and redacts every value.
	headers bool
}

var (
	headersKeep = &keepSet{headers: true}
	rotateKeep  = &keepSet{keys: scalarKeys("enabled", "max_size", "max_files", "max_age", "compress")}

	commonKeepKeys = []string{"type", "name", "log_level", "group_wait", "group_threshold", "max_retry", "timeout", "format"}

	// keepSets maps a plugin type to its allowlist. Keys that hold credentials are deliberately
	// absent; the table is documentation of what is safe, not of what is secret.
	keepSets = map[string]*keepSet{
		"dummy": pluginKeep(nil, "output_file"),
		"email": pluginKeep(map[string]shape{"receiver_emails": shapeScalarList},
			"smtp_host", "smtp_port", "sender_email", "sender_name", "email_subject", "encryption_type",
			"auth_type", "helo_host", "connect_timeout", "send_timeout"),
		"file": pluginKeep(map[string]shape{"rotate": shapeRotate}, "log_path"),
		"http": pluginKeep(map[string]shape{"headers": shapeHeaders},
			"unix_socket", "skip_tls_verification", "method", "cert_path", "key_path", "ca_cert_path"),
		"sentinel": pluginKeep(nil, "log_type"),
		"slack":    pluginKeep(nil, "channel", "username", "icon_emoji"),
		"splunk":   pluginKeep(nil),
	}
	commonOnlyKeep = pluginKeep(nil)
)

func scalarKeys(names ...string) map[string]shape {
	m := make(map[string]shape, len(names))
	for _, n := range names {
		m[n] = shapeScalar
	}
	return m
}

func pluginKeep(special map[string]shape, scalars ...string) *keepSet {
	m := scalarKeys(commonKeepKeys...)
	for _, n := range scalars {
		m[n] = shapeScalar
	}
	for k, s := range special {
		m[k] = s
	}
	return &keepSet{keys: m}
}

type visitKey struct {
	n  *yaml.Node
	ks *keepSet
}

// redactor rewrites document trees in place. Replacement is monotonic: a node once replaced stays
// replaced, so a node reachable through several paths ends up redacted if any path calls for it.
type redactor struct {
	visited  map[visitKey]struct{}
	replaced []string
}

// RedactNotificationFile rewrites a CrowdSec notification file so it keeps its structure but not its
// sensitive values. Everything not known to be harmless is replaced (fail closed). Comments are
// dropped, tags are cleared and anchors are renamed. On any doubt it returns ErrRedactUnsafe and the
// caller must not fall back to src.
func RedactNotificationFile(src []byte) (RedactResult, error) {
	if int64(len(src)) > maxInspectEntryBytes {
		return RedactResult{}, unsafeRedact("file too large")
	}
	docs, parseErr := ParseYAMLDocuments(bytes.NewReader(src), false)
	if parseErr != nil {
		return RedactResult{}, unsafeRedact("file cannot be parsed")
	}
	if len(docs) == 0 {
		return RedactResult{Data: []byte{}}, nil
	}
	nodes := 0
	for _, d := range docs {
		nodes += countNodes(d)
		if nodes > maxRedactNodes {
			return RedactResult{}, unsafeRedact("too many nodes")
		}
	}

	r := &redactor{visited: map[visitKey]struct{}{}}
	for _, d := range docs {
		r.document(d)
	}
	for _, d := range docs {
		if err := scrub(d); err != nil {
			return RedactResult{}, err
		}
	}
	count := 0
	for _, d := range docs {
		count += countPlaceholders(d)
	}
	if count > 0 && len(docs) > 0 {
		docs[0].HeadComment = "# " + redactHeaderComment
	}

	out, encodeErr := encodeDocuments(docs)
	if encodeErr != nil {
		return RedactResult{}, encodeErr
	}
	if err := checkResidue(docs, r.replaced, out); err != nil {
		return RedactResult{}, err
	}
	if _, err := ParseYAMLDocuments(bytes.NewReader(out), false); err != nil {
		return RedactResult{}, unsafeRedact("output does not parse")
	}
	return RedactResult{Data: out, Redacted: count}, nil
}

func countNodes(n *yaml.Node) int {
	total := 1
	for _, c := range n.Content {
		total += countNodes(c)
	}
	return total
}

func countPlaceholders(n *yaml.Node) int {
	total := 0
	if n.Kind == yaml.ScalarNode && n.Value == RedactedPlaceholder {
		total++
	}
	for _, c := range n.Content {
		total += countPlaceholders(c)
	}
	return total
}

// document redacts one document. A root that is not a mapping is replaced as a whole.
func (r *redactor) document(doc *yaml.Node) {
	if len(doc.Content) != 1 {
		return
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		r.value(root, false)
		return
	}
	r.mapping(root, keepSetFor(root))
}

// keepSetFor selects the allowlist from a direct, plain, non-aliased string "type" of the root.
func keepSetFor(root *yaml.Node) *keepSet {
	for i := 0; i+1 < len(root.Content); i += 2 {
		k, v := root.Content[i], root.Content[i+1]
		if k.Kind != yaml.ScalarNode || k.Value != "type" || k.Tag != "!!str" {
			continue
		}
		if v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Style == 0 {
			if ks, ok := keepSets[v.Value]; ok {
				return ks
			}
		}
		break
	}
	return commonOnlyKeep
}

func (r *redactor) mapping(m *yaml.Node, ks *keepSet) {
	vk := visitKey{m, ks}
	if _, seen := r.visited[vk]; seen {
		return
	}
	r.visited[vk] = struct{}{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Tag == "!!merge" {
			r.merge(v, ks)
			continue
		}
		if ks.headers {
			r.value(v, true)
			continue
		}
		sh, ok := ks.keys[k.Value]
		if k.Kind != yaml.ScalarNode || !ok {
			r.value(v, true)
			continue
		}
		r.keep(v, sh)
	}
}

// merge walks the mapping(s) a merge key pulls in within the merging mapping's context.
func (r *redactor) merge(v *yaml.Node, ks *keepSet) {
	switch target := resolveAlias(v); target.Kind {
	case yaml.MappingNode:
		r.mapping(target, ks)
	case yaml.SequenceNode:
		for _, item := range target.Content {
			if m := resolveAlias(item); m.Kind == yaml.MappingNode {
				r.mapping(m, ks)
			} else {
				r.value(item, true)
			}
		}
	default:
		r.value(v, true)
	}
}

// keep applies the expected shape of a kept key; any other shape is replaced as a whole.
func (r *redactor) keep(v *yaml.Node, sh shape) {
	switch sh {
	case shapeScalar:
		if resolveAlias(v).Kind == yaml.ScalarNode {
			return
		}
	case shapeScalarList:
		if v.Kind == yaml.SequenceNode && allScalars(v.Content) {
			return
		}
	case shapeHeaders:
		if v.Kind == yaml.MappingNode {
			r.mapping(v, headersKeep)
			return
		}
	case shapeRotate:
		if v.Kind == yaml.MappingNode {
			r.mapping(v, rotateKeep)
			return
		}
	}
	r.value(v, true)
}

func allScalars(nodes []*yaml.Node) bool {
	for _, n := range nodes {
		if n.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// value replaces a node in a redacted position. A mapping or sequence becomes one placeholder scalar,
// so no nested key or value survives. Kept as they are: a truly empty null, a placeholder already
// present and (with allowEnv) a pure environment reference.
func (r *redactor) value(n *yaml.Node, allowEnv bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value == "" && n.Tag == "!!null" {
			return
		}
		if n.Value == RedactedPlaceholder || (allowEnv && IsEnvReference(n.Value)) {
			return
		}
		r.note(n.Value)
	case yaml.MappingNode, yaml.SequenceNode:
		r.noteSubtree(n)
	}
	setPlaceholder(n)
}

func setPlaceholder(n *yaml.Node) {
	n.Kind = yaml.ScalarNode
	n.Content = nil
	n.Alias = nil
	n.Tag = "!!str"
	n.Value = RedactedPlaceholder
	n.Style = 0
}

// note remembers a replaced value for the residual scan.
func (r *redactor) note(v string) {
	if len(v) >= minCanaryLen && v != RedactedPlaceholder {
		r.replaced = append(r.replaced, v)
	}
}

func (r *redactor) noteSubtree(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode {
		r.note(n.Value)
	}
	for _, c := range n.Content {
		r.noteSubtree(c)
	}
}

// scrub clears comments and tags on every node and renames anchors to a1, a2, ... in document order,
// rewriting alias names to match. An alias whose anchor is no longer part of the document (its
// definition was replaced) cannot be written, so the file is unsafe.
func scrub(doc *yaml.Node) error {
	names := map[*yaml.Node]string{}
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		n.HeadComment, n.LineComment, n.FootComment = "", "", ""
		n.Tag = ""
		if n.Anchor != "" {
			n.Anchor = fmt.Sprintf("a%d", len(names)+1)
			names[n] = n.Anchor
		}
		if n.Kind == yaml.AliasNode {
			name, ok := names[n.Alias]
			if !ok {
				return unsafeRedact("alias without definition")
			}
			n.Value = name
		}
		for _, c := range n.Content {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(doc)
}

func encodeDocuments(docs []*yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, d := range docs {
		if err := enc.Encode(d); err != nil {
			return nil, unsafeRedact("output cannot be encoded")
		}
	}
	if err := enc.Close(); err != nil {
		return nil, unsafeRedact("output cannot be encoded")
	}
	return buf.Bytes(), nil
}

// checkResidue is the last line of defence: no replaced value of at least minCanaryLen characters may
// appear in the encoded output. Values that equal a scalar kept in the output (for example a sender
// address that is also the login name) and the text Charon writes itself are not leaks.
func checkResidue(docs []*yaml.Node, replaced []string, out []byte) error {
	if len(replaced) == 0 {
		return nil
	}
	kept := map[string]struct{}{
		RedactedPlaceholder: {}, redactHeaderComment: {}, strings.TrimSpace(RedactStubContent): {},
	}
	for _, d := range docs {
		collectScalars(d, kept)
	}
	unique := map[string]struct{}{}
	for _, v := range replaced {
		if _, ok := kept[v]; !ok {
			unique[v] = struct{}{}
		}
	}
	if len(unique)*len(out) > maxCanaryWork {
		return unsafeRedact("output too large to verify")
	}
	for v := range unique {
		if bytes.Contains(out, []byte(v)) {
			return unsafeRedact("residual value in output")
		}
	}
	return nil
}

func collectScalars(n *yaml.Node, into map[string]struct{}) {
	if n.Kind == yaml.ScalarNode {
		into[n.Value] = struct{}{}
	}
	for _, c := range n.Content {
		collectScalars(c, into)
	}
}
