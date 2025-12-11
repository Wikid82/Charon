package crowdsec

// Preset represents a curated CrowdSec preset offered by Charon.
type Preset struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Source      string   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
	RequiresHub bool     `json:"requires_hub"`
}

var curatedPresets = []Preset{
	{
		Slug:        "honeypot-friendly-defaults",
		Title:       "Honeypot Friendly Defaults",
		Summary:     "Lightweight parser and collection set tuned to reduce noise for tarpits and honeypots.",
		Source:      "charon-curated",
		Tags:        []string{"low-noise", "ssh", "http"},
		RequiresHub: false,
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
		Slug:        "geolocation-aware",
		Title:       "Geolocation Aware",
		Summary:     "Adds geo-aware decisions to tighten access by region; best paired with existing ACLs.",
		Source:      "charon-curated",
		Tags:        []string{"geo", "access-control"},
		RequiresHub: false,
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
