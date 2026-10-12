package crowdsec

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestListCuratedPresetsReturnsCopy(t *testing.T) {
	t.Parallel()
	got := ListCuratedPresets()
	if len(got) == 0 {
		t.Fatalf("expected curated presets, got none")
	}

	// mutate the copy and ensure originals stay intact on subsequent calls
	got[0].Title = "mutated"
	again := ListCuratedPresets()
	if again[0].Title == "mutated" {
		t.Fatalf("expected curated presets to be returned as copy, but mutation leaked")
	}
}

func TestFindPreset(t *testing.T) {
	t.Parallel()
	preset, ok := FindPreset("honeypot-friendly-defaults")
	if !ok {
		t.Fatalf("expected to find curated preset")
	}
	if preset.Slug != "honeypot-friendly-defaults" {
		t.Fatalf("unexpected preset slug %s", preset.Slug)
	}
	if preset.Title == "" {
		t.Fatalf("expected preset to have a title")
	}
	if preset.Summary == "" {
		t.Fatalf("expected preset to have a summary")
	}

	if _, ok := FindPreset("missing"); ok {
		t.Fatalf("expected missing preset to return ok=false")
	}
}

func TestFindPresetCaseVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		slug  string
		found bool
	}{
		{"exact match", "crowdsecurity/base-http-scenarios", true},
		{"another preset", "geoip-enrichment", true},
		{"case sensitive miss", "BOT-MITIGATION-ESSENTIALS", false},
		{"partial match miss", "bot-mitigation", false},
		{"empty slug", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, ok := FindPreset(tt.slug)
			if ok != tt.found {
				t.Errorf("FindPreset(%q) found=%v, want %v", tt.slug, ok, tt.found)
			}
		})
	}
}

func TestListCuratedPresetsReturnsDifferentCopy(t *testing.T) {
	t.Parallel()
	list1 := ListCuratedPresets()
	list2 := ListCuratedPresets()

	if len(list1) == 0 {
		t.Fatalf("expected non-empty preset list")
	}

	// Verify mutating one copy doesn't affect the other
	list1[0].Title = "MODIFIED"
	if list2[0].Title == "MODIFIED" {
		t.Fatalf("expected independent copies but mutation leaked")
	}

	// Verify subsequent calls return fresh copies
	list3 := ListCuratedPresets()
	if list3[0].Title == "MODIFIED" {
		t.Fatalf("mutation leaked to fresh copy")
	}
}

func TestCuratedPresetsValidate(t *testing.T) {
	t.Parallel()
	for _, p := range ListCuratedPresets() {
		if p.RequiresHub {
			continue
		}
		if err := p.Validate(); err != nil {
			t.Errorf("curated preset %q failed validation: %v", p.Slug, err)
		}
	}
}

func TestPresetValidateRejectsBadDefinitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		items []PresetItem
	}{
		{"empty items", nil},
		{"bad type traversal", []PresetItem{{Type: "../x", Name: "crowdsecurity/sshd"}}},
		{"unknown type", []PresetItem{{Type: "appsec-rules", Name: "crowdsecurity/sshd"}}},
		{"name with space", []PresetItem{{Type: "collections", Name: "a b"}}},
		{"name with shell meta", []PresetItem{{Type: "collections", Name: ";rm/x"}}},
		{"name traversal", []PresetItem{{Type: "collections", Name: "../x/y"}}},
		{"name without author", []PresetItem{{Type: "collections", Name: "sshd"}}},
		{"uppercase name", []PresetItem{{Type: "collections", Name: "CrowdSecurity/sshd"}}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Preset{Slug: "x", Items: tt.items}.Validate()
			if !errors.Is(err, ErrInvalidPresetDefinition) {
				t.Fatalf("expected ErrInvalidPresetDefinition, got %v", err)
			}
		})
	}
}

func TestPresetItemsNotSerialized(t *testing.T) {
	t.Parallel()
	p, ok := FindPreset("honeypot-friendly-defaults")
	if !ok || len(p.Items) == 0 {
		t.Fatalf("expected curated preset with items")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, found := m["Items"]; found {
		t.Fatalf("Items must not be serialized")
	}
	if _, found := m["items"]; found {
		t.Fatalf("items must not be serialized")
	}
}
