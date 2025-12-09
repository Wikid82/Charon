package crowdsec

import "testing"

func TestListCuratedPresetsReturnsCopy(t *testing.T) {
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
	preset, ok := FindPreset("honeypot-friendly-defaults")
	if !ok {
		t.Fatalf("expected to find curated preset")
	}
	if preset.Slug != "honeypot-friendly-defaults" {
		t.Fatalf("unexpected preset slug %s", preset.Slug)
	}

	if _, ok := FindPreset("missing"); ok {
		t.Fatalf("expected missing preset to return ok=false")
	}
}
