package core

import "testing"

func TestInvitationDomains(t *testing.T) {
	tests := []struct {
		host, kind, slug string
		valid            bool
	}{
		{"alvaroylaura.save.thedate.now", "wedding", "alvaroylaura", true},
		{"cumpleanoslucas.thedate.now", "general", "cumpleanoslucas", true},
		{"save.thedate.now", "", "", false},
		{"thedate.now", "", "", false},
		{"api.thedate.now", "", "", false},
		{"a.b.thedate.now", "", "", false},
		{"bad_slug.thedate.now", "", "", false},
	}
	for _, tt := range tests {
		kind, slug, ok := EventFromHost(tt.host)
		if kind != tt.kind || slug != tt.slug || ok != tt.valid {
			t.Errorf("EventFromHost(%q) = %q, %q, %v", tt.host, kind, slug, ok)
		}
	}
	for _, tt := range tests[:2] {
		got, err := InvitationHost(tt.kind, tt.slug)
		if err != nil || got != tt.host {
			t.Errorf("InvitationHost(%q, %q) = %q, %v", tt.kind, tt.slug, got, err)
		}
	}
}
