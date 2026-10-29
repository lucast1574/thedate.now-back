package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRetryOnlyReleasesRejectedUnsentMessages(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rows := []any{
		bson.M{"_id": "failed", "eventId": "event", "sendState": "failed", "sendClaim": "rejected", "sendStartedAt": now},
		bson.M{"_id": "unknown", "eventId": "event", "sendState": "unknown", "sendClaim": "uncertain"},
		bson.M{"_id": "sending", "eventId": "event", "sendState": "sending", "sendClaim": "inflight"},
		bson.M{"_id": "sent", "eventId": "event", "sendState": "failed", "sentAt": now, "sendClaim": "delivered"},
		bson.M{"_id": "other", "eventId": "other", "sendState": "failed", "sendClaim": "other"},
	}
	if _, err := db.Collection("guests").InsertMany(ctx, rows); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	for i := 0; i < 2; i++ {
		if err := s.retryRejectedInvitations(ctx, "event"); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"failed", "unknown", "sending", "sent", "other"} {
		var g bson.M
		if err := db.Collection("guests").FindOne(ctx, bson.M{"_id": id}).Decode(&g); err != nil {
			t.Fatal(err)
		}
		_, claimed := g["sendClaim"]
		if claimed != (id != "failed") {
			t.Fatalf("unexpected claim for %s", id)
		}
		if id == "failed" && (g["sendState"] != nil || g["sendStartedAt"] != nil) {
			t.Fatal("rejected send was not reset")
		}
	}
}

func TestWhatsAppReadinessRequiresExactWorkingSession(t *testing.T) {
	for _, item := range []struct {
		name, status string
		code         int
		ready        bool
	}{
		{"test", "WORKING", 200, true}, {"test", "SCAN_QR_CODE", 200, false},
		{"test", "FAILED", 200, false}, {"other", "WORKING", 200, false},
		{"test", "WORKING", 404, false},
	} {
		provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/api/sessions/test" || r.Header.Get("X-Api-Key") != "key" {
				t.Error("invalid readiness request")
			}
			w.WriteHeader(item.code)
			_ = json.NewEncoder(w).Encode(map[string]string{"name": item.name, "status": item.status})
		}))
		previous := wazendClient
		wazendClient = provider.Client()
		got := wazendSessionReady(context.Background(), wazendSettings{Base: provider.URL, Key: "key", Session: "test"})
		wazendClient = previous
		provider.Close()
		if got != item.ready {
			t.Fatalf("%s/%s/%d readiness=%v", item.name, item.status, item.code, got)
		}
	}
}
