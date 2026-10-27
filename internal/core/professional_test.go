package core

import "testing"

func TestThreeProductRoles(t *testing.T) {
	for _, kind := range []string{"general", "wedding"} {
		p, _ := ProductFor(kind)
		for _, role := range []string{"user", p.Role, "admin"} {
			if !ValidProductRole(kind, role) || !UserCanCreate(kind, User{Role: role, Product: kind}) {
				t.Fatal("missing product role", kind, role)
			}
		}
		if CanShareEvent(kind, User{Role: "user"}) || ValidProductRole(kind, "couple") {
			t.Fatal("retired or ordinary role can share")
		}
		if !CanShareEvent(kind, User{Role: p.Role}) || !CanShareEvent(kind, User{Role: "admin"}) {
			t.Fatal("professional cannot share")
		}
	}
	if ValidRole("couple") {
		t.Fatal("retired role accepted")
	}
}
