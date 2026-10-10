package crowdsec

import (
	"os"
	"path/filepath"
	"strings"
)

// IsHubPath reports whether rel (a slash- or OS-separated path relative to DataDir) lies in the
// installed hub tree: hub/ at the top level or config/hub/. The tree is a regenerable cache managed by
// cscli, and installed items are links into it, so it is treated as server-local state. Matching is
// case-sensitive and a nested directory merely named "hub" stays configuration.
func IsHubPath(rel string) bool {
	slash := filepath.ToSlash(filepath.Clean(rel))
	if slash == "." || slash == "" {
		return false
	}
	first, rest, _ := strings.Cut(slash, "/")
	if first == "hub" {
		return true
	}
	second, _, _ := strings.Cut(rest, "/")
	return first == "config" && second == "hub"
}

// IsExportExcluded reports whether rel is left out of a configuration export: preserved state plus
// the installed hub tree. Imports keep the same set, so the two rules cannot drift apart.
func IsExportExcluded(rel string) bool {
	return IsPreservedPath(rel) || IsHubPath(rel)
}

// ImportKeeper returns the predicate an import uses to decide which entries of dataDir stay in place:
// preserved state, the installed hub tree and every symlink that currently exists in the live tree.
// The symlink check reads the live tree at call time.
func ImportKeeper(dataDir string) func(rel string) bool {
	return func(rel string) bool {
		return IsPreservedPath(rel) || IsHubPath(rel) || isLiveSymlink(dataDir, rel)
	}
}

// isLiveSymlink reports whether rel names a symlink below dataDir. The path is lstat-ed, never
// followed; unsafe or missing paths are simply not symlinks.
//
// Invariant: Lstat resolves symlinks in the intermediate components, so for a path beneath a live
// link the answer describes the link's target, not a link inside the tree. The result is only
// meaningful when every ancestor of rel has already been checked and found not to be kept, which is
// what callers guarantee by visiting ancestors first (KeptOrBeneathKept runs the shortest prefix
// first; the clear and copy walks never descend into a kept entry).
func isLiveSymlink(dataDir, rel string) bool {
	if rel == "" || strings.ContainsRune(rel, 0) || !filepath.IsLocal(rel) {
		return false
	}
	info, err := os.Lstat(filepath.Join(dataDir, rel))
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// KeptOrBeneathKept reports whether keep holds for rel or for any of its ancestor directories.
func KeptOrBeneathKept(keep func(rel string) bool, rel string) bool {
	slash := filepath.ToSlash(filepath.Clean(rel))
	parts := strings.Split(slash, "/")
	for i := range parts {
		if keep(filepath.FromSlash(strings.Join(parts[:i+1], "/"))) {
			return true
		}
	}
	return false
}
