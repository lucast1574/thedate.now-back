package httpapi

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/lucast1574/thedate.now-back/internal/mongostore"
)

func TestSupportRetriesAndConcurrentAdminUpdates(t *testing.T) {
	s, tokens := supportFixture(t)
	in := supportDraft(t)
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := supportCall(s, tokens["owner"], "POST", "/support/tickets", in)
			var ticket supportView
			if (w.Code != 200 && w.Code != 201) || json.Unmarshal(w.Body.Bytes(), &ticket) != nil {
				t.Errorf("retry: %d %s", w.Code, w.Body.String())
				return
			}
			ids <- ticket.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for next := range ids {
		if id != "" && id != next {
			t.Fatal("retry created duplicate tickets")
		}
		id = next
	}
	count, _ := s.db.Collection("supportTickets").CountDocuments(context.Background(), map[string]string{})
	if count != 1 || id == "" {
		t.Fatal("duplicate report stored")
	}
	codes := make(chan int, 2)
	for _, state := range []string{"IN_PROGRESS", "WAITING"} {
		wg.Add(1)
		go func(state string) {
			defer wg.Done()
			codes <- supportCall(s, tokens["admin"], "PATCH", "/admin/support/tickets/"+id, map[string]any{"state": state, "version": 0, "note": "Estamos revisando este problema."}).Code
		}(state)
	}
	wg.Wait()
	close(codes)
	winners, conflicts := 0, 0
	for code := range codes {
		if code == 200 {
			winners++
		}
		if code == 409 {
			conflicts++
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatal("concurrent admin update overwrote another")
	}
	w := supportCall(s, tokens["admin"], "PATCH", "/admin/support/tickets/"+id, map[string]any{"state": "DONE", "version": 1, "note": "El problema ya está corregido."})
	if w.Code != 200 {
		t.Fatalf("resolve: %d %s", w.Code, w.Body.String())
	}
	ticket, err := (mongostore.SupportTickets{DB: s.db}).Get(context.Background(), id, "owner", false)
	if err != nil || ticket.State != "DONE" || ticket.ResolvedAt == nil || ticket.Version != 2 || len(ticket.History) != 3 || ticket.History[2].Note != "El problema ya está corregido." {
		t.Fatal("history or resolution missing")
	}
	if supportCall(s, tokens["admin"], "PATCH", "/admin/support/tickets/"+id, map[string]any{"state": "OPEN", "version": 2}).Code != 400 {
		t.Fatal("resolved ticket reopened")
	}
}

func TestSupportRateLimitPreservesSuccessfulRetries(t *testing.T) {
	s, tokens := supportFixture(t)
	first := supportDraft(t)
	if supportCall(s, tokens["owner"], "POST", "/support/tickets", first).Code != 201 {
		t.Fatal("first report failed")
	}
	for range 4 {
		in := supportDraft(t)
		in.Images = nil
		if supportCall(s, tokens["owner"], "POST", "/support/tickets", in).Code != 201 {
			t.Fatal("expected report failed")
		}
	}
	if w := supportCall(s, tokens["owner"], "POST", "/support/tickets", supportDraft(t)); w.Code != 429 {
		t.Fatalf("rate limit missing: %d", w.Code)
	}
	if supportCall(s, tokens["owner"], "POST", "/support/tickets", first).Code != 200 {
		t.Fatal("successful retry rejected after rate limit")
	}
}
