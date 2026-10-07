package core

import "testing"

func TestProfileValidation(t *testing.T) {
	for _, name := range []string{"x", "Alice\nSmith", ""} {
		if _, ok := ProfileName(name); ok {
			t.Fatal("invalid name allowed")
		}
	}
	if name, ok := ProfileName("  Sofía Mateo  "); !ok || name != "Sofía Mateo" {
		t.Fatal("valid name rejected")
	}
	for _, raw := range []string{"http://lh3.googleusercontent.com/a", "https://googleusercontent.com.attacker.test/a", "https://localhost/a", "https://a:secret@lh3.googleusercontent.com/a"} {
		if GooglePhoto(raw) != "" {
			t.Fatal("untrusted photo URL allowed")
		}
	}
	if GooglePhoto("https://lh3.googleusercontent.com/a/photo") == "" {
		t.Fatal("Google photo rejected")
	}
}
