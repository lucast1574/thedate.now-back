package httpapi

import (
	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"net/http"
	"regexp"
	"time"
)

func validInstallation(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}

var fidPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{20,256}$`)

func (s *server) registerMobileDevice(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	var in struct {
		InstallationID string `json:"installationId"`
		FID            string `json:"fid"`
		Platform       string `json:"platform"`
	}
	if decode(r, &in) != nil || !validInstallation(in.InstallationID) || !fidPattern.MatchString(in.FID) || (in.Platform != "android" && in.Platform != "ios") {
		bad(w, 400, "Invalid device")
		return
	}
	if s.mobileSender == nil {
		bad(w, 503, "Notifications are not configured")
		return
	}
	device := core.MobileDevice{ID: in.InstallationID, UserID: u.ID, FID: in.FID, Platform: in.Platform, TokenVersion: u.TokenVersion}
	if err = (mongostore.MobileDevices{DB: s.db}).Register(r.Context(), device, time.Now().UTC()); err != nil {
		bad(w, 500, "Could not register device")
		return
	}
	reply(w, 200, map[string]bool{"registered": true})
}
func (s *server) removeMobileDevice(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	id := r.PathValue("installation")
	if !validInstallation(id) {
		bad(w, 400, "Invalid device")
		return
	}
	if err = (mongostore.MobileDevices{DB: s.db}).Remove(r.Context(), id, u.ID); err != nil {
		bad(w, 500, "Could not remove device")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
