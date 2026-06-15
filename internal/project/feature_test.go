package project

import "testing"

func TestStatusValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   Status
		want bool
	}{
		{"pending", StatusPending, true},
		{"in_progress", StatusInProgress, true},
		{"implemented", StatusImplemented, true},
		{"failed", StatusFailed, true},
		{"empty", Status(""), false},
		{"unknown", Status("done"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Valid(); got != tt.want {
				t.Errorf("Status(%q).Valid() = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Config Loader", "config-loader"},
		{"already-slug", "config-loader", "config-loader"},
		{"punctuation", "pave init: scaffold!", "pave-init-scaffold"},
		{"leading-trailing", "  --Hello--  ", "hello"},
		{"collapse-dashes", "a   ---   b", "a-b"},
		{"numbers", "Phase 1 Foundation", "phase-1-foundation"},
		{"empty", "", ""},
		{"symbols-only", "!!!@@@", ""},
		{"unicode-stripped", "café münchen", "caf-m-nchen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Slug(tt.in); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSlugUniquenessForDistinctTitles(t *testing.T) {
	t.Parallel()
	a := Slug("Config Loader")
	b := Slug("State Store")
	if a == b {
		t.Errorf("expected distinct slugs, both = %q", a)
	}
}
