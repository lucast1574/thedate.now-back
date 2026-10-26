package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type supportInput struct {
	RequestID   string   `json:"requestId"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
}
type supportRequester struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
type supportView struct {
	core.SupportTicket
	Screenshots []string          `json:"images"`
	Requester   *supportRequester `json:"requester,omitempty"`
}

func ticketView(ticket core.SupportTicket, admin bool) supportView {
	view := supportView{SupportTicket: ticket, Screenshots: []string{}}
	for i := range ticket.Images {
		view.Screenshots = append(view.Screenshots, "/api/support/"+ticket.ID+"/images/"+strconv.Itoa(i))
	}
	if admin {
		view.Requester = &supportRequester{Name: ticket.RequesterName, Email: ticket.RequesterEmail}
	}
	return view
}
func (s *server) createSupportTicket(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var in supportInput
	if decoder.Decode(&in) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		bad(w, 400, "Invalid ticket")
		return
	}
	requestID, err := uuid.Parse(in.RequestID)
	if err != nil {
		bad(w, 400, "Invalid submission identifier")
		return
	}
	in.RequestID = requestID.String()
	title, description, err := core.SupportText(in.Title, in.Description)
	if err != nil {
		bad(w, 400, "Usa un asunto de 3 a 140 caracteres y una descripción de 10 a 10 000 caracteres.")
		return
	}
	repo := mongostore.SupportTickets{DB: s.db}
	prior, err := repo.Prior(r.Context(), u.ID, in.RequestID)
	if err == nil {
		reply(w, 200, ticketView(prior, false))
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		bad(w, 500, "Could not verify submission")
		return
	}
	allowed, err := mongostore.Allow(r.Context(), s.db, "support:"+u.ID, 5, time.Now().UTC())
	if err != nil {
		bad(w, 503, "Ticket submission unavailable")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		bad(w, 429, "Has enviado varios reportes. Espera un minuto antes de volver a intentarlo.")
		return
	}
	images, err := supportImages(in.Images)
	if err != nil {
		bad(w, 400, "Adjunta hasta tres capturas PNG, JPEG o WebP de 1 MB como máximo.")
		return
	}
	now := time.Now().UTC()
	ticket := core.SupportTicket{Mail: map[string]core.EmailDelivery{"ticket_received": core.PendingEmail(now)}, ID: uuid.NewString(), RequesterID: u.ID, RequesterName: u.Name, RequesterEmail: u.Email, RequestID: in.RequestID, Title: title, Description: description, State: "OPEN", Images: images, History: []core.SupportChange{{State: "OPEN", At: now, ActorID: u.ID}}, CreatedAt: now, UpdatedAt: now}
	ticket, err = repo.Create(r.Context(), ticket)
	if err != nil {
		bad(w, 500, "Could not save ticket")
		return
	}
	reply(w, 201, ticketView(ticket, false))
}
func (s *server) listSupportTickets(w http.ResponseWriter, r *http.Request) {
	s.supportList(w, r, false)
}
func (s *server) adminSupportTickets(w http.ResponseWriter, r *http.Request) {
	s.supportList(w, r, true)
}
func (s *server) supportList(w http.ResponseWriter, r *http.Request, admin bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Sign in required")
		return
	}
	if admin && u.Role != "admin" {
		bad(w, 403, "Administrator required")
		return
	}
	filter := mongostore.SupportFilter{State: r.URL.Query().Get("state")}
	if !admin {
		filter.RequesterID = u.ID
	}
	if filter.State != "" && !core.ValidSupportState(filter.State) {
		bad(w, 400, "Invalid ticket state")
		return
	}
	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 64)
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		bad(w, 400, "Invalid page")
		return
	}
	tickets, total, err := (mongostore.SupportTickets{DB: s.db}).List(r.Context(), filter, page)
	if err != nil {
		bad(w, 500, "Could not load tickets")
		return
	}
	views := make([]supportView, 0, len(tickets))
	for _, ticket := range tickets {
		views = append(views, ticketView(ticket, admin))
	}
	reply(w, 200, map[string]any{"items": views, "total": total, "page": page, "pages": (total + 19) / 20})
}
func (s *server) updateSupportTicket(w http.ResponseWriter, r *http.Request) {
	u, ok := s.admin(w, r)
	if !ok {
		return
	}
	var in struct {
		State   string `json:"state"`
		Note    string `json:"note"`
		Version int64  `json:"version"`
	}
	if decode(r, &in) != nil || in.Version < 0 {
		bad(w, 400, "Invalid update")
		return
	}
	repo := mongostore.SupportTickets{DB: s.db}
	ticket, err := repo.Get(r.Context(), r.PathValue("id"), "", false)
	if err != nil {
		bad(w, 404, "Ticket not found")
		return
	}
	if ticket.Version != in.Version {
		bad(w, 409, "El reporte cambió. Actualiza la bandeja antes de guardarlo.")
		return
	}
	change, err := core.SupportTransition(ticket, in.State, in.Note, u.ID, time.Now().UTC())
	if err != nil {
		bad(w, 400, "Elige otro estado o añade una respuesta. Los reportes resueltos no se pueden modificar.")
		return
	}
	ticket, err = repo.Change(r.Context(), ticket, change)
	if err != nil {
		bad(w, 409, "Otro administrador actualizó este reporte. Vuelve a cargarlo.")
		return
	}
	reply(w, 200, ticketView(ticket, true))
}
