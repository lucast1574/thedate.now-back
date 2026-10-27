package httpapi

import "testing"

func TestStripeLiveKeyModes(t *testing.T) {
	for _, tt := range []struct {
		key  string
		live bool
	}{{"sk_live_example", true}, {"rk_live_example", true}, {"sk_test_example", false}, {"", false}} {
		t.Run(tt.key, func(t *testing.T) {
			t.Setenv("STRIPE_SECRET_KEY", tt.key)
			if stripeLive() != tt.live {
				t.Fatal("incorrect payment mode")
			}
		})
	}
}
