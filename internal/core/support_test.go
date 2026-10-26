package core

import (
	"strings"
	"testing"
	"time"
)

func TestSupportValidationAndTerminalStates(t *testing.T) {
	title, body, err := SupportText("  No se guardó  ", "  La invitación sigue sin guardar los cambios.  ")
	if err != nil || title != "No se guardó" || body != "La invitación sigue sin guardar los cambios." {
		t.Fatal("ticket text not normalized")
	}
	for _, in := range [][2]string{{"Hi", "A long enough report"}, {strings.Repeat("á", 141), "A long enough report"}, {"Title", "small"}, {"Title", strings.Repeat("x", 10001)}} {
		if _, _, err := SupportText(in[0], in[1]); err == nil {
			t.Fatal("invalid ticket accepted")
		}
	}
	open := SupportTicket{State: "OPEN", History: []SupportChange{{State: "OPEN"}}}
	if _, err := SupportTransition(open, "NOT_REAL", "", "admin", time.Now()); err == nil {
		t.Fatal("invalid state accepted")
	}
	if _, err := SupportTransition(open, "OPEN", "", "admin", time.Now()); err == nil {
		t.Fatal("empty update accepted")
	}
	if change, err := SupportTransition(open, "OPEN", "  Estamos revisándolo.  ", "admin", time.Now()); err != nil || change.Note != "Estamos revisándolo." {
		t.Fatal("reply without state change rejected")
	}
	if _, err := SupportTransition(SupportTicket{State: "DONE"}, "OPEN", "Reopen", "admin", time.Now()); err == nil {
		t.Fatal("resolved ticket reopened")
	}
}
