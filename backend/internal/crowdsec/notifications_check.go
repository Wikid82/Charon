package crowdsec

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	// maxListedNotifications bounds how many undefined names an error message spells out.
	maxListedNotifications = 10
	// maxListedNameLen bounds one listed name; the archive is untrusted.
	maxListedNameLen = 64

	defaultConfigDir       = "/etc/crowdsec"
	defaultProfilesPath    = "/etc/crowdsec/profiles.yaml"
	defaultNotificationDir = "/etc/crowdsec/notifications"
)

// UndefinedNotificationsError reports profiles that name notifications no file in the archive defines.
type UndefinedNotificationsError struct {
	// Names are the referenced-but-undefined notification names, sorted.
	Names []string
}

// NameList spells out the undefined names, bounded in count and length and safe to log or show.
func (e *UndefinedNotificationsError) NameList() string {
	listed := e.Names
	more := 0
	if len(listed) > maxListedNotifications {
		more = len(listed) - maxListedNotifications
		listed = listed[:maxListedNotifications]
	}
	safe := make([]string, len(listed))
	for i, n := range listed {
		n = util.SanitizeForLog(n)
		if r := []rune(n); len(r) > maxListedNameLen {
			n = string(r[:maxListedNameLen]) + "..."
		}
		safe[i] = n
	}
	msg := strings.Join(safe, ", ")
	if more > 0 {
		msg += fmt.Sprintf(", and %d more", more)
	}
	return msg
}

func (e *UndefinedNotificationsError) Error() string {
	return "profiles.yaml names notifications that have no matching file: " + e.NameList()
}

// VerifyInspectedNotifications returns an *UndefinedNotificationsError when the inspected archive's
// profiles name notifications that none of its notification files defines. Cases the archive cannot
// settle (see CheckProfileNotifications) pass.
func VerifyInspectedNotifications(info ArchiveInfo) error {
	if missing := CheckProfileNotifications(info); len(missing) > 0 {
		return &UndefinedNotificationsError{Names: missing}
	}
	return nil
}

// CheckProfileNotifications returns the sorted notification names that profiles.yaml references and no
// notification file in the archive defines. CrowdSec keys notification configs by the name: field of
// every YAML document in the lowercase .yaml and .yml files directly inside notifications/, and refuses
// to start when a profile names one that is missing. Only the archive is considered: an import removes
// the live profiles and notification files before extracting.
//
// The check passes (returns nil) instead of guessing when the archive cannot settle the question:
// incomplete inspection, no profiles file, a config that redirects the profiles file or notification
// directory, a profiles or notification file that does not parse, or a notifications value that is not
// a list of names.
func CheckProfileNotifications(info ArchiveInfo) []string {
	if info.Incomplete {
		return skipNotificationCheck("an inspected entry exceeded a bound")
	}
	prefix := ""
	if _, ok := info.Files["config/config.yaml"]; ok {
		prefix = "config/"
	}
	profiles, ok := info.Files[prefix+"profiles.yaml"]
	if !ok {
		return skipNotificationCheck("no profiles file in the archive")
	}
	if configRedirectsPaths(info.Files[prefix+"config.yaml"]) {
		return skipNotificationCheck("config redirects the profiles or notification paths")
	}
	referenced, ok := referencedNotifications(profiles)
	if !ok || len(referenced) == 0 {
		return skipNotificationCheck("profiles reference no notifications or cannot be read")
	}
	defined, ok := definedNotifications(info.Files, prefix+"notifications")
	if !ok {
		return skipNotificationCheck("a notification file cannot be read")
	}
	var missing []string
	for name := range referenced {
		if !defined[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func skipNotificationCheck(reason string) []string {
	logger.Log().WithField("reason", reason).Debug("crowdsec import: notification reference check skipped")
	return nil
}

// configRedirectsPaths reports whether config.yaml (empty when absent) moves the config directory, the
// profiles file or the notification directory away from the defaults, or cannot be read: the archive
// layout then does not say where CrowdSec looks.
func configRedirectsPaths(config []byte) bool {
	if len(config) == 0 {
		return false
	}
	docs, err := ParseYAMLDocuments(bytes.NewReader(config), true)
	if err != nil {
		return true
	}
	if len(docs) == 0 {
		return false
	}
	root := documentMapping(docs[0])
	for _, p := range []struct {
		def  string
		keys []string
	}{
		{defaultConfigDir, []string{"config_paths", "config_dir"}},
		{defaultNotificationDir, []string{"config_paths", "notification_dir"}},
		{defaultProfilesPath, []string{"api", "server", "profiles_path"}},
	} {
		node, found := lookupPath(root, p.keys)
		if !found {
			continue
		}
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || path.Clean(node.Value) != p.def {
			return true
		}
	}
	return false
}

// lookupPath follows nested mapping keys. It reports false when a key is absent; a non-mapping on the
// way is returned as the found node so the caller treats it as unexpected.
func lookupPath(m *yaml.Node, keys []string) (*yaml.Node, bool) {
	cur := m
	for _, k := range keys {
		if cur == nil || cur.Kind != yaml.MappingNode {
			return cur, cur != nil
		}
		cur = mappingValue(cur, k)
		if cur == nil {
			return nil, false
		}
	}
	return cur, true
}

// referencedNotifications collects the names listed under notifications: in every profile document.
// It reports false when the file does not parse or a value is not a list of non-empty strings.
func referencedNotifications(profiles []byte) (map[string]bool, bool) {
	docs, err := ParseYAMLDocuments(bytes.NewReader(profiles), false)
	if err != nil {
		return nil, false
	}
	names := map[string]bool{}
	for _, doc := range docs {
		v := mappingValue(documentMapping(doc), "notifications")
		if v == nil || (v.Kind == yaml.ScalarNode && v.Tag == "!!null") {
			continue
		}
		if v.Kind != yaml.SequenceNode {
			return nil, false
		}
		for _, item := range v.Content {
			item = resolveAlias(item)
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Value == "" {
				return nil, false
			}
			names[item.Value] = true
		}
	}
	return names, true
}

// definedNotifications collects name: from every document of every notification file CrowdSec loads
// from dir. It reports false when any such file does not parse.
func definedNotifications(files map[string][]byte, dir string) (map[string]bool, bool) {
	byFile, ok := definedNotificationsByFile(files, dir)
	if !ok {
		return nil, false
	}
	defined := map[string]bool{}
	for _, names := range byFile {
		for _, n := range names {
			defined[n] = true
		}
	}
	return defined, true
}

// definedNotificationsByFile maps every notification file in dir to the names its documents define (one
// entry per document, so a name repeated in a file is repeated here). It reports false when any such
// file does not parse.
func definedNotificationsByFile(files map[string][]byte, dir string) (map[string][]string, bool) {
	byFile := map[string][]string{}
	for rel, data := range files {
		if path.Dir(rel) != dir {
			continue
		}
		docs, err := ParseYAMLDocuments(bytes.NewReader(data), false)
		if err != nil {
			return nil, false
		}
		names := []string{}
		for _, doc := range docs {
			if n := mappingValue(documentMapping(doc), "name"); n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
				names = append(names, n.Value)
			}
		}
		byFile[rel] = names
	}
	return byFile, true
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// documentMapping returns the top-level mapping of a document node, or nil.
func documentMapping(doc *yaml.Node) *yaml.Node {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	if m := resolveAlias(doc.Content[0]); m.Kind == yaml.MappingNode {
		return m
	}
	return nil
}

// mappingValue returns the value node for key in a mapping (nil when absent or m is not a mapping).
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return resolveAlias(m.Content[i+1])
		}
	}
	return nil
}
