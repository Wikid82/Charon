package crowdsec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"

	"gopkg.in/yaml.v3"
)

const (
	// mirrorConfigPrefix starts the names of the short-lived cscli configuration copies that point cscli at the
	// hub mirror (see mirrorCSCLIConfig). IsHubMirrorConfig recognises them.
	mirrorConfigPrefix = ".charon-hub-mirror-"
	// mirrorTemplateKey is cscli's hidden hub URL template key. It is a test hook of cscli 1.8.2, not a documented
	// option, so the contract doc records it.
	mirrorTemplateKey = "__hub_url_template__"
	mirrorBranchKey   = "hub_branch"
	mirrorCSCLISec    = "cscli"
	// mirrorDefaultBranch is the hub branch cscli is pinned to when the configuration names none.
	mirrorDefaultBranch = "master"
	// builtinMirrorTemplate is the template of the built-in mirror: the GitHub hub mirror, branch and file.
	builtinMirrorTemplate = "https://raw.githubusercontent.com/crowdsecurity/hub/%s/%s"
	// mirrorMemoryTTL is how long requests go mirror-first after the mirror answered one. A hung hub then costs one
	// attempt per job instead of one per item.
	mirrorMemoryTTL = 10 * time.Minute
)

var (
	// errMirrorTemplateSet means the live configuration already names a hub URL template. The retry must not
	// replace an operator's own hub, so it is refused.
	errMirrorTemplateSet = errors.New("the cscli config already sets a hub URL template")
	// errMirrorTemplateInvalid means the template is not an https URL with exactly the two verbs cscli fills in.
	errMirrorTemplateInvalid = errors.New("the hub mirror template must be an https URL with exactly two %s verbs")
	// createMirrorTemp creates the derived configuration file. It is a variable so tests can make creation fail.
	createMirrorTemp = os.CreateTemp
)

// MirrorMemory remembers for a short window that the hub mirror answered a cscli hub request, so later requests
// in that window go mirror-first. The window starts at the first win and is not extended by later wins; Forget
// closes it when the live configuration answers again. A nil *MirrorMemory is always inactive.
type MirrorMemory struct {
	mu    sync.Mutex
	ttl   time.Duration
	until time.Time
	now   func() time.Time
}

// NewMirrorMemory returns a memory whose window lasts ttl. A nil clock means time.Now.
func NewMirrorMemory(ttl time.Duration, now func() time.Time) *MirrorMemory {
	return &MirrorMemory{ttl: ttl, now: now}
}

func (m *MirrorMemory) clock() time.Time {
	if m.now == nil {
		return time.Now()
	}
	return m.now()
}

// Active reports whether the mirror-first window is open.
func (m *MirrorMemory) Active() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clock().Before(m.until)
}

// Remember opens the window for ttl after a mirror win. A win while the window is already open changes nothing.
func (m *MirrorMemory) Remember() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if now := m.clock(); !now.Before(m.until) {
		m.until = now.Add(m.ttl)
	}
}

// Forget closes the window.
func (m *MirrorMemory) Forget() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.until = time.Time{}
}

// isHubNetworkOp reports whether cscli arguments name an operation that downloads from the hub: `hub update`,
// `hub upgrade`, and `<type> install` or `<type> upgrade`. Local reads such as `inspect` and `list` are not.
func isHubNetworkOp(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if args[0] == "hub" {
		return args[1] == "update" || args[1] == "upgrade"
	}
	return args[1] == "install" || args[1] == "upgrade"
}

// transportTimeoutMarkers are the output fragments of an HTTP request that timed out inside the client. They name a
// request that never got an answer, which a retry on another address can fix.
var transportTimeoutMarkers = []string{"client.timeout", "tls handshake timeout"}

// deadlineMarker is the text of an expired context. On its own it can be a healthy slow install that ran out of its
// time, so only a hub update retries on it.
const deadlineMarker = "context deadline exceeded"

// isHubUpdate reports whether args are `hub update`.
func isHubUpdate(args []string) bool {
	return len(args) >= 2 && args[0] == "hub" && args[1] == "update"
}

// isMirrorRetryableFailure reports whether cscli output names a network failure or a transport timeout. An item
// error (unknown, tainted) and a bare deadline are not retryable here.
func isMirrorRetryableFailure(text string) bool {
	if hasNetworkFailure(text) {
		return true
	}
	lower := strings.ToLower(text)
	for _, marker := range transportTimeoutMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// isRetryableHubFailure reports whether cscli output names a failure a hub update can retry on the other address: a
// network failure, a transport timeout, or an expired deadline. Other failures (an unknown item, a tainted item) are
// not retried.
func isRetryableHubFailure(text string) bool {
	return isMirrorRetryableFailure(text) || strings.Contains(strings.ToLower(text), deadlineMarker)
}

// mirrorHubTemplate returns the cscli hub URL template for a mirror base, such as
// https://raw.githubusercontent.com/crowdsecurity/hub/master. The second result reports whether the operator's base
// was used. An operator base is used only when it passes validateHubURL and ends in the master branch that cscli
// is pinned to; otherwise the built-in mirror is used and a warning names the reason. An empty base means the
// built-in mirror without a warning.
func mirrorHubTemplate(mirrorBase string) (string, bool) {
	base := strings.TrimRight(strings.TrimSpace(mirrorBase), "/")
	if base == "" {
		return builtinMirrorTemplate, false
	}
	template, refusal := operatorMirrorTemplate(base)
	if refusal != "" {
		logger.Log().WithField("hub_mirror", HubURLForLog(base)).WithField("reason", refusal).
			Warn("hub mirror base not usable, using the built-in mirror")
		return builtinMirrorTemplate, false
	}
	return template, true
}

// operatorMirrorTemplate turns an operator mirror base into a template, or names why it cannot be used.
func operatorMirrorTemplate(base string) (template, refusal string) {
	if err := validateHubURL(base); err != nil {
		return "", "not an allowlisted https hub address"
	}
	prefix, branch := path.Split(base)
	if branch != mirrorDefaultBranch {
		return "", "the address does not end in the master branch"
	}
	template = strings.TrimSuffix(prefix, "/") + "/%s/%s"
	if !validMirrorTemplate(template) {
		return "", "the address cannot form a valid template"
	}
	return template, ""
}

// validMirrorTemplate reports whether template is an https URL with exactly two %s verbs and no other percent
// sign, and no control characters. Any other template could make cscli format something unintended.
func validMirrorTemplate(template string) bool {
	return strings.HasPrefix(template, "https://") &&
		strings.Count(template, "%s") == 2 &&
		strings.Count(template, "%") == 2 &&
		!strings.ContainsAny(template, "\r\n\x00")
}

// mirrorCSCLIConfig writes a copy of the cscli configuration at configPath whose cscli section points the hub at
// template, and returns the copy's path with a cleanup that removes it. The copy is built with yaml.v3 (never by
// text concatenation), created with mode 0600 in the same directory so relative paths in it resolve as before, and
// a sibling config.yaml.local is copied next to it because CrowdSec merges that file by name. A live configuration
// that already sets a template is refused with errMirrorTemplateSet, and an unreadable local override fails the
// call, so the caller never retries with a configuration it cannot reproduce. On error the cleanup is a no-op.
// The copy holds the live settings, so IsHubMirrorConfig keeps it out of listings, reads, exports and imports.
func mirrorCSCLIConfig(configPath, template string) (derivedPath string, cleanup func(), err error) {
	noop := func() {}
	if configPath == "" {
		return "", noop, errors.New("cscli config path is empty")
	}
	if !validMirrorTemplate(template) {
		return "", noop, errMirrorTemplateInvalid
	}
	live := filepath.Clean(configPath)
	raw, err := os.ReadFile(live)
	if err != nil {
		return "", noop, fmt.Errorf("read cscli config: %w", err)
	}
	content, err := withMirrorTemplate(raw, template)
	if err != nil {
		return "", noop, err
	}
	f, err := createMirrorTemp(filepath.Dir(live), mirrorConfigPrefix+"*.yaml")
	if err != nil {
		return "", noop, fmt.Errorf("create hub mirror config: %w", err)
	}
	derived := f.Name()
	cleanup = func() {
		_ = os.Remove(derived)
		_ = os.Remove(derived + ".local")
	}
	if err := writeMirrorFile(f, content); err != nil {
		cleanup()
		return "", noop, err
	}
	if err := copyLocalOverride(live+".local", derived+".local"); err != nil {
		cleanup()
		return "", noop, err
	}
	return derived, cleanup, nil
}

// withMirrorTemplate returns the YAML of a cscli configuration with the hub URL template set in its cscli section.
// The section is created when missing, and hub_branch is set to master when it is missing or empty. Every other
// key is kept.
func withMirrorTemplate(raw []byte, template string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse cscli config: %w", err)
	}
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("cscli config root is not a mapping")
	}
	section, err := childMapping(root, mirrorCSCLISec)
	if err != nil {
		return nil, err
	}
	if i := findMappingKey(section, mirrorTemplateKey); i >= 0 && !isEmptyScalar(section.Content[i+1]) {
		return nil, errMirrorTemplateSet
	}
	setMappingValue(section, mirrorTemplateKey, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: template, Style: yaml.DoubleQuotedStyle})
	if i := findMappingKey(section, mirrorBranchKey); i < 0 || isEmptyScalar(section.Content[i+1]) {
		setMappingValue(section, mirrorBranchKey, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mirrorDefaultBranch})
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("encode hub mirror config: %w", err)
	}
	return out, nil
}

// childMapping returns the mapping under key in m, creating an empty one when the key is missing or null. Any
// other kind of value is an error.
func childMapping(m *yaml.Node, key string) (*yaml.Node, error) {
	i := findMappingKey(m, key)
	if i < 0 {
		value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
		return value, nil
	}
	value := m.Content[i+1]
	if isNullScalar(value) {
		value.Kind, value.Tag, value.Value, value.Style = yaml.MappingNode, "!!map", "", 0
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cscli config %s is not a mapping", key)
	}
	return value, nil
}

// findMappingKey returns the index of key's key node in the mapping m, or -1. Keys sit at even positions of Content.
func findMappingKey(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// setMappingValue replaces the value under key in the mapping m, or appends the key when it is missing.
func setMappingValue(m *yaml.Node, key string, value *yaml.Node) {
	if i := findMappingKey(m, key); i >= 0 {
		m.Content[i+1] = value
		return
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

// isNullScalar reports whether n is a YAML null (an empty value or ~).
func isNullScalar(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// isEmptyScalar reports whether n is a null or an empty string.
func isEmptyScalar(n *yaml.Node) bool {
	return isNullScalar(n) || (n.Kind == yaml.ScalarNode && n.Value == "")
}

// writeMirrorFile writes data into the file f and closes it. The file was created with mode 0600 by
// os.CreateTemp, so the content is never readable by other users.
func writeMirrorFile(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write hub mirror config: %w", err)
	}
	return nil
}

// copyLocalOverride copies the CrowdSec local override src to dst, when src exists. dst is created exclusively with
// mode 0600. A missing override is not an error; an unreadable one is, so the caller does not retry without it.
func copyLocalOverride(src, dst string) error {
	raw, err := os.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read cscli local override: %w", err)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create hub mirror local override: %w", err)
	}
	return writeMirrorFile(f, raw)
}

// isMirrorConfigName reports whether a lower-case file name is a derived mirror configuration or its local copy.
func isMirrorConfigName(lower string) bool {
	return strings.HasPrefix(lower, mirrorConfigPrefix) &&
		(strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yaml.local"))
}

// sweepMirrorLeftovers removes derived mirror configurations that a crash left in the data directory or its config
// directory. It returns an error only when a leftover cannot be removed; a missing directory is not an error.
func sweepMirrorLeftovers(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	var errs []error
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "config")} {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("list hub mirror leftovers: %w", err))
			continue
		}
		for _, e := range entries {
			if !isMirrorConfigName(strings.ToLower(e.Name())) {
				continue
			}
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				errs = append(errs, fmt.Errorf("remove hub mirror leftover: %w", err))
			}
		}
	}
	return errors.Join(errs...)
}
