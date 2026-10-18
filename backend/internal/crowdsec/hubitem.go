package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// HubItemTypes are the hub item types an export records and a restore can install. It is wider than the
// types Charon's own presets use (allowedPresetItemTypes).
var HubItemTypes = []string{"collections", "contexts", "parsers", "postoverflows", "scenarios"}

var (
	// hubItemNamePattern accepts every real hub item name, mixed case included (for example
	// "Azlaroc/sftpgo", "crowdsecurity/CVE-2022-26134"). ASCII only, a single slash, no leading dash.
	hubItemNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}/[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	// presetItemNamePattern is the stricter lowercase form Charon uses for the presets it authors.
	presetItemNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)
)

// IsHubItemType reports whether t is one of HubItemTypes.
func IsHubItemType(t string) bool {
	return slices.Contains(HubItemTypes, t)
}

// ValidHubItemName reports whether name is a well-formed hub item name in owner/name form. A name that
// passes can never be read as a flag, contains no whitespace or shell metacharacters and cannot
// traverse a path.
func ValidHubItemName(name string) bool {
	return hubItemNamePattern.MatchString(name) && !strings.Contains(name, "..")
}

// ValidPresetItemName is ValidHubItemName restricted to the lowercase names Charon's presets use.
func ValidPresetItemName(name string) bool {
	return ValidHubItemName(name) && presetItemNamePattern.MatchString(name)
}

// HubItem identifies one installed hub item by type and name. Versions are not recorded: they cannot be
// recovered from a link and cscli cannot pin them.
type HubItem struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// Valid reports whether the item has a supported type and a well-formed name.
func (i HubItem) Valid() bool {
	return IsHubItemType(i.Type) && ValidHubItemName(i.Name)
}

// compareHubItems orders items by type, then name.
func compareHubItems(a, b HubItem) int {
	if c := strings.Compare(a.Type, b.Type); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// VerifyHubItem requires `cscli <type> inspect <name> -o json` to report the item as installed and
// untainted. configPath is the engine configuration file passed to cscli with -c ("" for none). Callers validate
// itemType and name first.
func VerifyHubItem(ctx context.Context, exec CommandExecutor, configPath, itemType, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("verify %s %s: %w", itemType, name, err)
	}
	out, err := RunCSCLI(ctx, exec, configPath, itemType, "inspect", name, "-o", "json")
	if err != nil {
		return err
	}
	// Field names follow cscli's `-o json` inspect output (installed, tainted).
	var state struct {
		Installed *bool `json:"installed"`
		Tainted   bool  `json:"tainted"`
	}
	if err := unmarshalCSCLIJSON(out, &state); err != nil {
		return fmt.Errorf("parse inspect output: %w", err)
	}
	if state.Installed == nil {
		return errors.New("inspect output has no installed field")
	}
	if !*state.Installed {
		return errors.New("item reported as not installed")
	}
	if state.Tainted {
		return errors.New("item reported as tainted")
	}
	return nil
}
