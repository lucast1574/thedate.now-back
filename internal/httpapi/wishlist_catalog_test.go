package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/testutil"
	"go.mongodb.org/mongo-driver/v2/bson"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicCatalogNeverRequiresActivationOrLeaksGuestClaims(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	e := core.Event{ID: "event", Kind: "wedding", Slug: "boda", PaymentStatus: "paid", PublishedAt: &now}
	db.Collection("events").InsertOne(ctx, e)
	s := &server{db: db}
	call := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	w := call("/public/events/wedding/boda/wishlist")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal(w.Code, w.Body.String())
	}
	e.Wishlist.Items = []core.Gift{{ID: "gift", Title: "Cafetera", URL: "https://shop.example/p", GuestID: "private-id"}}
	db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"wishlist": e.Wishlist}})
	w = call("/public/events/wedding/boda/wishlist")
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-id") || strings.Contains(w.Body.String(), `"mine":true`) || !strings.Contains(w.Body.String(), `"reserved":true`) {
		t.Fatal(w.Body.String())
	}
	if call("/public/events/general/boda/wishlist").Code != 404 {
		t.Fatal("cross product")
	}
	db.Collection("events").UpdateByID(ctx, e.ID, bson.M{"$set": bson.M{"paymentStatus": "unpaid"}})
	if call("/public/events/wedding/boda/wishlist").Code != 404 {
		t.Fatal("unpublished list leaked")
	}
}
func TestGiftAccessSendsOnlyToRegisteredPhoneAndDoesNotReturnBearer(t *testing.T) {
	db := testutil.Mongo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	var sent atomic.Int32
	token := strings.Repeat("a", 48)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["chatId"] != "51999111222@c.us" || !strings.Contains(body["text"].(string), "/para/ana-") || !strings.HasSuffix(body["text"].(string), "/regalos?elegir=gift") {
			t.Error(body)
		}
		sent.Add(1)
		w.WriteHeader(200)
	}))
	defer provider.Close()
	previous := wazendClient
	wazendClient = provider.Client()
	defer func() { wazendClient = previous }()
	t.Setenv("WAZEND_WEDDING_BASE_URL", provider.URL)
	t.Setenv("WAZEND_WEDDING_API_KEY", "test")
	t.Setenv("WAZEND_WEDDING_SESSION", "test")
	db.Collection("events").InsertOne(ctx, core.Event{ID: "event", Kind: "wedding", Slug: "boda", PaymentStatus: "paid", PublishedAt: &now, Wishlist: core.Wishlist{Items: []core.Gift{{ID: "gift", Title: "Cafetera", URL: "https://shop.example/p"}}}})
	db.Collection("guests").InsertOne(ctx, core.Guest{ID: "guest", EventID: "event", Name: "Ana", Phone: "+51999111222", InviteToken: token})
	s := &server{db: db}
	call := func(phone string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("POST", "/public/events/wedding/boda/wishlist/access", strings.NewReader(`{"phone":"`+phone+`","giftId":"gift"}`)))
		return w
	}
	for _, phone := range []string{"+51999111222", "+51999111222", "+51999888777"} {
		w := call(phone)
		if w.Code != 202 || strings.Contains(w.Body.String(), token) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if sent.Load() != 1 {
		t.Fatal("duplicate or unregistered message", sent.Load())
	}
}
