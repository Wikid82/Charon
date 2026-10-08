package crowdsec

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidPresetDefinition is returned when a curated preset definition fails validation.
var ErrInvalidPresetDefinition = errors.New("invalid preset definition")

// PresetItem is a single CrowdSec hub item installed by a curated preset.
type PresetItem struct {
	// Type is the cscli hub type (collections, parsers, scenarios, postoverflows).
	Type string
	// Name is the hub item name in author/name form.
	Name string
}

var (
	allowedPresetItemTypes = map[string]struct{}{
		"collections":   {},
		"parsers":       {},
		"scenarios":     {},
		"postoverflows": {},
	}
	presetItemNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)
)

// Preset represents a curated CrowdSec preset offered by Charon.
//
// RequiresHub=false marks a preset defined by Charon itself (a server-owned bundle
// of hub items listed in Items) rather than one fetched from the hub index.
type Preset struct {
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Summary     string       `json:"summary"`
	Source      string       `json:"source"`
	Tags        []string     `json:"tags,omitempty"`
	RequiresHub bool         `json:"requires_hub"`
	Items       []PresetItem `json:"-"`
}

// Validate checks that the preset's item list is non-empty and every item uses an
// allowlisted hub type and a well-formed author/name.
func (p Preset) Validate() error {
	if len(p.Items) == 0 {
		return fmt.Errorf("%w: preset %q has no items", ErrInvalidPresetDefinition, p.Slug)
	}
	for _, item := range p.Items {
		if _, ok := allowedPresetItemTypes[item.Type]; !ok {
			return fmt.Errorf("%w: unsupported item type %q", ErrInvalidPresetDefinition, item.Type)
		}
		if !presetItemNamePattern.MatchString(item.Name) {
			return fmt.Errorf("%w: invalid item name %q", ErrInvalidPresetDefinition, item.Name)
		}
	}
	return nil
}

var curatedPresets = []Preset{
	{
		Slug:        "honeypot-friendly-defaults",
		Title:       "Honeypot Friendly Defaults",
		Summary:     "Installs SSH and Caddy log parsing with brute-force and web probing detection, plus the CrowdSec whitelists parser.",
		Source:      "charon-curated",
		Tags:        []string{"ssh", "http"},
		RequiresHub: false,
		Items: []PresetItem{
			{Type: "collections", Name: "crowdsecurity/sshd"},
			{Type: "collections", Name: "crowdsecurity/caddy"},
			{Type: "scenarios", Name: "crowdsecurity/http-backdoors-attempts"},
			{Type: "scenarios", Name: "crowdsecurity/http-probing"},
			{Type: "parsers", Name: "crowdsecurity/whitelists"},
		},
	},
	{
		Slug:        "crowdsecurity/base-http-scenarios",
		Title:       "Bot Mitigation Essentials",
		Summary:     "Core scenarios for bad bots and credential stuffing with minimal false positives (maps to base-http-scenarios).",
		Source:      "hub",
		Tags:        []string{"bots", "auth", "web"},
		RequiresHub: true,
	},
	{
		Slug:        "geoip-enrichment",
		Title:       "GeoIP Enrichment",
		Summary:     "Enriches CrowdSec log events with GeoIP data (country and ASN). It does not block or allow traffic by region.",
		Source:      "charon-curated",
		Tags:        []string{"geo", "enrichment"},
		RequiresHub: false,
		Items: []PresetItem{
			{Type: "parsers", Name: "crowdsecurity/geoip-enrich"},
		},
	},
}

// ListCuratedPresets returns a copy of curated presets to avoid external mutation.
func ListCuratedPresets() []Preset {
	out := make([]Preset, len(curatedPresets))
	copy(out, curatedPresets)
	return out
}

// FindPreset returns a preset by slug.
func FindPreset(slug string) (Preset, bool) {
	for _, p := range curatedPresets {
		if p.Slug == slug {
			return p, true
		}
	}
	return Preset{}, false
}
