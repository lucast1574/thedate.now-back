package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClosedResponseNoticeIsDeduplicatedAndNeverChangesAttendance(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message map[string]string
		json.NewDecoder(r.Body).Decode(&message)
		if !strings.Contains(message["text"], "no modifica tu confirmaci") || !strings.HasSuffix(message["text"], "/respuesta") {
			t.Error("Notice must explain the lock and link to saved response")
		}
		calls.Add(1)
		w.WriteHeader(200)
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_WEDDING_BASE_URL", provider.URL)
	t.Setenv("WAZEND_WEDDING_API_KEY", "test")
	t.Setenv("WAZEND_WEDDING_SESSION", "test")
	db := testutil.Mongo(t)
	s := &server{db: db}
	ctx := context.Background()
	now := time.Now()
	g := core.Guest{ID: "guest", EventID: "event", InviteToken: strings.Repeat("a", 48)}
	e := core.Event{ID: "event", Kind: "wedding", Slug: "boda", PaymentStatus: "paid", PublishedAt: &now, ResponseVersion: 1, Responses: map[string]core.Response{g.ID: {Choice: "going", Finalized: true, Seats: 1, ProviderTimestamp: 100}}}
	db.Collection("events").InsertOne(ctx, e)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.notifyClosedResponse(ctx, e.ID, g, "not_going", "same-webhook", 101); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	s.notifyClosedResponse(ctx, e.ID, g, "going", "same-choice", 102)
	s.notifyClosedResponse(ctx, e.ID, g, "not_going", "old-webhook", 99)
	if calls.Load() != 1 {
		t.Fatalf("Duplicate or unnecessary notices: %d", calls.Load())
	}
	current, _ := s.responses().Repository.Snapshot(ctx, e.ID)
	if current.Responses[g.ID].Choice != "going" {
		t.Fatal("Notice changed attendance")
	}
}
