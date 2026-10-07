package core

import "testing"

func TestSessionMethodBindsIdentityWithoutRequiringGoogleForDelegatedAdmins(t *testing.T) {
	for _, test := range []struct {
		name        string
		user        User
		method, pin string
		allowed     bool
	}{
		{"delegated password admin", User{Email: "delegate@example.test", Role: "admin"}, "password", "", true},
		{"delegated cannot invent Google identity", User{Email: "delegate@example.test", Role: "admin"}, "google", "", false},
		{"linked Google blocks password", User{Email: "delegate@example.test", Role: "admin", GoogleSub: "linked", IdentityPolicy: 1}, "password", "", false},
		{"delegated verified Google", User{Email: "delegate@example.test", Role: "admin", GoogleSub: "linked", IdentityPolicy: 1}, "google", "", true},
		{"legacy Google must refresh", User{Email: "delegate@example.test", Role: "admin", GoogleSub: "linked"}, "google", "", false},
		{"owner rejects password", User{Email: "owner@gmail.com", Role: "admin"}, "password", "", false},
		{"owner rejects untrusted Google", User{Email: "owner@gmail.com", Role: "admin", GoogleSub: "other", IdentityPolicy: 1}, "google", "trusted", false},
		{"owner accepts pinned identity", User{Email: "owner@gmail.com", Role: "admin", GoogleSub: "trusted", IdentityPolicy: 1}, "google", "trusted", true},
		{"owner accepts authoritative identity", User{Email: "owner@gmail.com", Role: "admin", GoogleSub: "trusted", IdentityPolicy: 1, GoogleAuthoritative: true}, "google", "", true},
		{"unknown method denied", User{Email: "delegate@example.test", Role: "admin"}, "unknown", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := SessionMethodAllowed(test.user, test.method, "owner@gmail.com", test.pin); got != test.allowed {
				t.Fatalf("allowed=%v, want %v", got, test.allowed)
			}
		})
	}
}
