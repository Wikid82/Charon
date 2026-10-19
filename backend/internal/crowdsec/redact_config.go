package crowdsec

import (
	"bytes"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// configFileRels are the engine configuration files that can hold credentials, in the live locations of
// both layouts: the data folder itself or its config folder.
var configFileRels = []string{"config.yaml", "config.yaml.local", "config/config.yaml", "config/config.yaml.local"}

// isConfigFile reports whether the slash path is the engine configuration or its local override.
func isConfigFile(slash string) bool {
	for _, rel := range configFileRels {
		if slash == rel {
			return true
		}
	}
	return false
}

// credentialKeyNames are the key names whose scalar value is a credential wherever they appear (compared
// case-insensitively).
var credentialKeyNames = map[string]struct{}{
	"password": {}, "passwd": {}, "pwd": {}, "passphrase": {}, "secret": {}, "token": {}, "api_key": {}, "apikey": {}, "private_key": {},
	"authorization": {}, "bearer": {}, "psk": {}, "dsn": {}, "connection_string": {},
}

// credentialKeySuffixes and credentialKeyParts widen the exact names to compound ones such as db_password,
// webhook_token or client_secret_key: a lower-cased key that ends in a suffix or contains a part is a credential.
var (
	credentialKeySuffixes = []string{
		"password", "passwd", "secret", "token", "secret_key", "apikey", "api_key", "private_key",
		"passphrase", "access_key", "signing_key", "encryption_key", "_pwd", "_psk", "_dsn", "_connection_string",
	}
	credentialKeyParts = []string{"_password", "_token"}
)

// benignKeySuffixes and benignKeyNames exclude keys that look like credentials but hold something else: where a
// credential lives (a path, file or folder), or a limit. Without them a rule such as "contains _token" would
// blank settings like token_path or tokens_per_minute.
var (
	benignKeySuffixes = []string{"_path", "_file", "_dir", "_folder", "_url", "_uri", "_ttl", "_len", "_length", "_count", "_per_minute", "_per_second", "_enabled", "_expiry", "_seconds", "_interval", "_timeout"}
	benignKeyNames    = map[string]struct{}{"tokens_per_minute": {}, "tokens_per_second": {}, "max_tokens": {}}
)

// isCredentialLikeName reports whether a lower-cased key name is a compound credential name that the exact
// list does not cover and that is not a known benign setting.
func isCredentialLikeName(name string) bool {
	if _, ok := benignKeyNames[name]; ok {
		return false
	}
	for _, suffix := range benignKeySuffixes {
		if strings.HasSuffix(name, suffix) {
			return false
		}
	}
	for _, suffix := range credentialKeySuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, part := range credentialKeyParts {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

// credentialPaths are the keys that are credentials only below one parent (the dotted lower-case path of
// the parent): the database user and the registration token. The other credential names of the database
// block are covered by credentialKeyNames.
var credentialPaths = map[string][]string{
	"db_config":                    {"user", "password"},
	"api.server.auto_registration": {"token"},
}

// urlSchemePattern is the scheme syntax of RFC 3986.
var urlSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)

// aliasSensitiveKeys are the key names that make an alias or merge target unsafe to leave unread: the
// database user is a credential only by its path, which a shared node would hide.
var aliasSensitiveKeys = map[string]struct{}{"user": {}, "db_config": {}}

func isCredentialKey(parentPath, name string) bool {
	if _, ok := credentialKeyNames[name]; ok {
		return true
	}
	for _, k := range credentialPaths[parentPath] {
		if k == name {
			return true
		}
	}
	return isCredentialLikeName(name)
}

// configRedactor blanks credential values of a configuration tree in place and counts them. Nothing is
// modified unless a credential value is found.
type configRedactor struct {
	redactor
	blanked int
}

// RedactConfigFile blanks the credentials of a CrowdSec engine configuration (config.yaml or its local
// override): the database user and password, the registration token, any value under a credential key
// name (credentialKeyNames) or compound credential name (isCredentialLikeName) and the password part of a URL with userinfo. Everything else is kept (a
// denylist, because the file holds dozens of settings that must stay). A file without a credential value
// is returned as it is, comments included; the rewrite drops comments, so it only happens when it must.
// A file that cannot be rewritten safely (it does not parse, is too large or complex, or shares
// credential nodes through anchors or merge keys) returns ErrRedactUnsafe and the caller must not fall
// back to src.
func RedactConfigFile(src []byte) (RedactResult, error) {
	if int64(len(src)) > maxInspectEntryBytes {
		return RedactResult{}, unsafeRedact("file too large")
	}
	docs, parseErr := ParseYAMLDocuments(bytes.NewReader(src), false)
	if parseErr != nil {
		return RedactResult{}, unsafeRedact("file cannot be parsed")
	}
	nodes := 0
	for _, d := range docs {
		nodes += countNodes(d)
		if nodes > maxRedactNodes {
			return RedactResult{}, unsafeRedact("too many nodes")
		}
	}

	c := &configRedactor{}
	for _, d := range docs {
		if err := c.walk(d, ""); err != nil {
			return RedactResult{}, err
		}
	}
	if c.blanked == 0 {
		return RedactResult{Data: src}, nil
	}
	for _, d := range docs {
		// A shared node cannot be merged back on import, so a file that needs rewriting and uses them is out.
		if hasAliasOrMerge(d) {
			return RedactResult{}, unsafeRedact("anchors in a file with credentials")
		}
		if err := scrub(d); err != nil {
			return RedactResult{}, err
		}
	}
	docs[0].HeadComment = "# " + redactHeaderComment
	out, err := encodeDocuments(docs)
	if err != nil {
		return RedactResult{}, err
	}
	if err := checkResidue(docs, c.replaced, out); err != nil {
		return RedactResult{}, err
	}
	if _, err := ParseYAMLDocuments(bytes.NewReader(out), false); err != nil {
		return RedactResult{}, unsafeRedact("output does not parse")
	}
	count := 0
	for _, d := range docs {
		count += countConfigPlaceholders(d)
	}
	return RedactResult{Data: out, Redacted: count}, nil
}

// walk visits the tree below n; path is the dotted lower-case key path of n. Aliases are not followed:
// the node they name is visited where it is defined.
func (c *configRedactor) walk(n *yaml.Node, path string) error {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, child := range n.Content {
			if err := c.walk(child, path); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		return c.mapping(n, path)
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if err := c.item(item, path+".[]"); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		c.urlPassword(n)
	}
	return nil
}

// item walks a value that is not a credential: an alias is only checked for shared credentials.
func (c *configRedactor) item(v *yaml.Node, path string) error {
	if v.Kind == yaml.AliasNode {
		return checkAliasTarget(v)
	}
	return c.walk(v, path)
}

func (c *configRedactor) mapping(m *yaml.Node, path string) error {
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Tag == "!!merge" {
			if err := checkAliasTarget(v); err != nil {
				return err
			}
			continue
		}
		if k.Kind != yaml.ScalarNode {
			return unsafeRedact("complex key")
		}
		name := strings.ToLower(k.Value)
		if isCredentialKey(path, name) {
			if err := c.credential(v); err != nil {
				return err
			}
			continue
		}
		child := name
		if path != "" {
			child = path + "." + name
		}
		if err := c.item(v, child); err != nil {
			return err
		}
	}
	return nil
}

// checkAliasTarget refuses an alias or merge target that holds a key only its path marks as a credential.
func checkAliasTarget(v *yaml.Node) error {
	if subtreeHasKey(resolveAlias(v), aliasSensitiveKeys) {
		return unsafeRedact("shared node with credentials")
	}
	return nil
}

func subtreeHasKey(n *yaml.Node, names map[string]struct{}) bool {
	for i, child := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 {
			if _, ok := names[strings.ToLower(child.Value)]; ok {
				return true
			}
		}
		if subtreeHasKey(resolveAlias(child), names) {
			return true
		}
	}
	return false
}

// credential blanks the value of a credential key. Kept as they are: an empty or null value, a boolean (a
// switch such as require_password is not a secret), a placeholder already present and a pure environment
// reference.
func (c *configRedactor) credential(v *yaml.Node) error {
	switch v.Kind {
	case yaml.AliasNode:
		return unsafeRedact("alias at a credential")
	case yaml.ScalarNode:
		if v.Value == "" || v.Tag == "!!null" || v.Tag == "!!bool" || IsPlaceholder(v) || IsEnvReference(v.Value) {
			return nil
		}
	default:
		if len(v.Content) == 0 {
			return nil
		}
	}
	if v.Anchor != "" {
		return unsafeRedact("anchored credential")
	}
	c.noteSubtree(v)
	setPlaceholder(v)
	c.blanked++
	return nil
}

// urlPassword replaces the password of a URL scalar with userinfo.
func (c *configRedactor) urlPassword(n *yaml.Node) {
	if n.Tag != "!!str" {
		return
	}
	if redacted, password, ok := redactURLPassword(n.Value); ok {
		c.note(password)
		n.Value = redacted
		c.blanked++
	}
}

// redactURLPassword replaces the password in the userinfo of "scheme://user:password@host/..." with the
// placeholder. It reports the new text and the password it removed; ok is false when v is not such a URL
// or its password is empty, already the placeholder or an environment reference.
func redactURLPassword(v string) (redacted, password string, ok bool) {
	scheme, rest, found := strings.Cut(v, "://")
	if !found || !urlSchemePattern.MatchString(scheme) {
		return v, "", false
	}
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	authority := rest[:end]
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return v, "", false
	}
	user, password, hasPassword := strings.Cut(authority[:at], ":")
	if !hasPassword || password == "" || password == RedactedPlaceholder || IsEnvReference(password) {
		return v, "", false
	}
	return scheme + "://" + user + ":" + RedactedPlaceholder + authority[at:] + rest[end:], password, true
}
