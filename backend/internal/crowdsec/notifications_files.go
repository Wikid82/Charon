package crowdsec

import (
	"bytes"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// NotificationFileKind is the class IsNotificationFile assigns to a path of a configuration tree.
type NotificationFileKind int

const (
	// NotAFile is anything that is not treated as a notification file.
	NotAFile NotificationFileKind = iota
	// InDirYAML is a YAML file inside a notification directory.
	InDirYAML
	// InDirOther is a non-YAML file inside a notification directory; CrowdSec never reads it.
	InDirOther
	// LookAlikeYAML is a YAML file outside the notification directories whose root names a known
	// notification plugin type.
	LookAlikeYAML
)

// knownPluginTypes are the notification plugin types CrowdSec ships.
var knownPluginTypes = map[string]struct{}{
	"dummy": {}, "email": {}, "file": {}, "http": {}, "sentinel": {}, "slack": {}, "splunk": {},
}

// defaultNotificationDirs are the notification directories below the data folder for the flat and the
// nested layout.
var defaultNotificationDirs = []string{"notifications", "config/notifications"}

// contentDirs are the CrowdSec content directories of the config root. A redirected notification
// folder at or below one of them is ignored, so custom parsers and scenarios are never rewritten.
var contentDirs = map[string]struct{}{
	"parsers": {}, "scenarios": {}, "collections": {}, "postoverflows": {}, "contexts": {},
	"appsec-configs": {}, "appsec-rules": {}, "appsec-data": {}, "patterns": {}, "hub": {}, "data": {},
}

func hasYAMLExt(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".yaml", ".yml":
		return true
	}
	return false
}

// IsNotificationFile is the single place that decides what a notification file is. rel is a slash path
// below the root being examined and dirs is NotificationDirs for that same root. parsedRoot is the root
// mapping (or its document node) of the file's first document, or nil when the caller has not parsed
// it; it is only consulted for YAML files outside the notification directories. The function never
// reads files itself.
func IsNotificationFile(rel string, parsedRoot *yaml.Node, dirs []string) NotificationFileKind {
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return NotAFile
	}
	if inNotificationDir(rel, dirs) {
		if hasYAMLExt(rel) {
			return InDirYAML
		}
		return InDirOther
	}
	if hasYAMLExt(rel) && namesKnownPlugin(parsedRoot) {
		return LookAlikeYAML
	}
	return NotAFile
}

// inNotificationDir reports whether rel lies below a directory component named "notifications" (any
// case, any position) or below one of the exact configured directories in dirs.
func inNotificationDir(rel string, dirs []string) bool {
	parts := strings.Split(rel, "/")
	for _, p := range parts[:len(parts)-1] {
		if strings.EqualFold(p, "notifications") {
			return true
		}
	}
	for _, d := range dirs {
		if d != "" && strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// namesKnownPlugin reports whether root is a mapping with a direct plain string "type" naming a known
// plugin type.
func namesKnownPlugin(root *yaml.Node) bool {
	if root == nil {
		return false
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) != 1 {
			return false
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		k, v := root.Content[i], root.Content[i+1]
		if k.Kind == yaml.ScalarNode && k.Value == "type" {
			if v.Kind != yaml.ScalarNode || v.Tag != "!!str" || v.Style != 0 {
				return false
			}
			_, ok := knownPluginTypes[strings.ToLower(v.Value)]
			return ok
		}
	}
	return false
}

// NotificationDirs returns the notification directories of the tree below dataDir as slash paths
// relative to it: the two defaults plus the folder config_paths.notification_dir of configYAML names
// when it resolves safely inside the tree. A value that cannot be resolved statically or fails the
// safety rules is ignored and the defaults alone are returned. configYAML may be nil.
func NotificationDirs(dataDir string, configYAML []byte) []string {
	dirs := append([]string(nil), defaultNotificationDirs...)
	value, ok := configuredNotificationDir(configYAML)
	if !ok {
		return dirs
	}
	rel, ok := resolveNotificationDir(dataDir, value)
	if !ok {
		logger.Log().Debug("crowdsec notification folder setting ignored")
		return dirs
	}
	for _, d := range dirs {
		if d == rel {
			return dirs
		}
	}
	return append(dirs, rel)
}

// configuredNotificationDir reads config_paths.notification_dir from the first document of configYAML.
func configuredNotificationDir(configYAML []byte) (string, bool) {
	if len(configYAML) == 0 {
		return "", false
	}
	docs, err := ParseYAMLDocuments(bytes.NewReader(configYAML), true)
	if err != nil || len(docs) == 0 || len(docs[0].Content) != 1 {
		if err != nil {
			logger.Log().Debug("crowdsec config unreadable; using the default notification folders")
		}
		return "", false
	}
	node := mappingValue(docs[0].Content[0], "config_paths")
	node = mappingValue(node, "notification_dir")
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", false
	}
	return node.Value, true
}

// resolveNotificationDir maps a configured folder to a slash path below dataDir. Nothing is expanded
// (a "$" anywhere means it cannot be resolved statically) and no link is followed.
func resolveNotificationDir(dataDir, value string) (string, bool) {
	if value == "" || strings.ContainsRune(value, 0) || strings.Contains(value, "$") {
		return "", false
	}
	value = filepath.ToSlash(value)
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", false
		}
	}
	if !strings.HasPrefix(value, "/") {
		return "", false
	}
	value = path.Clean(value)
	root := path.Clean(filepath.ToSlash(filepath.Clean(dataDir)))
	var rel string
	switch {
	case value == root:
		return "", false
	case strings.HasPrefix(value, root+"/"):
		rel = strings.TrimPrefix(value, root+"/")
	case (value == "/etc/crowdsec" || strings.HasPrefix(value, "/etc/crowdsec/")) && DetectLayout(dataDir) == LayoutNested:
		rel = "config" + strings.TrimPrefix(value, "/etc/crowdsec")
	default:
		return "", false
	}
	if rel == "config" || rel == "" || isContentDirPath(rel) {
		return "", false
	}
	return rel, true
}

// isContentDirPath reports whether rel is at or below a CrowdSec content directory, directly below the
// data folder or below its config folder.
func isContentDirPath(rel string) bool {
	for _, p := range []string{rel, strings.TrimPrefix(rel, "config/")} {
		first, _, _ := strings.Cut(p, "/")
		if _, ok := contentDirs[strings.ToLower(first)]; ok {
			return true
		}
	}
	return false
}

// maxEnvRefBytes bounds the values the environment reference test looks at.
const maxEnvRefBytes = 4096

var envRefPattern = regexp.MustCompile(`^\$(\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*)$`)

// envRefSchemes are the header schemes that may precede a reference.
var envRefSchemes = []string{"Bearer ", "Basic ", "token ", "Token "}

// IsEnvReference reports whether v is nothing but one environment reference ("${NAME}" or "$NAME"),
// optionally behind one header scheme. A value with any other text, or longer than 4 KiB, is not one.
func IsEnvReference(v string) bool {
	if len(v) > maxEnvRefBytes {
		return false
	}
	for _, s := range envRefSchemes {
		if rest, ok := strings.CutPrefix(v, s); ok {
			return envRefPattern.MatchString(rest)
		}
	}
	return envRefPattern.MatchString(v)
}
