package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Each product owns an entire database, including accounts, referrals and events.
// The previous shared database remains untouched and is never a login fallback.
func isolatedServers(client *mongo.Client, base string, master []byte, storage *storage) ([]*server, error) {
	general, wedding := os.Getenv("GOOGLE_GENERAL_CLIENT_ID"), os.Getenv("GOOGLE_WEDDING_CLIENT_ID")
	if general == "" || wedding == "" || general == wedding {
		return nil, errors.New("isolated products require distinct Google clients")
	}
	names := []string{env("MONGODB_GENERAL_DATABASE", base+"_general"), env("MONGODB_WEDDING_DATABASE", base+"_wedding")}
	if names[0] == names[1] || names[0] == base || names[1] == base {
		return nil, errors.New("product databases must be independent of each other and the legacy database")
	}
	servers := make([]*server, 0, 2)
	for i, product := range []string{"general", "wedding"} {
		servers = append(servers, &server{product: product, db: client.Database(names[i]), secret: productSecret(master, product), storage: storage})
	}
	return servers, nil
}

func productSecret(master []byte, product string) []byte {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte("thedate-session-v1:" + product))
	return mac.Sum(nil)
}

func productRoutes(servers []*server) http.Handler {
	return securityHeaders(productHandler(servers))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

func productHandler(servers []*server) http.Handler {
	if len(servers) == 1 && servers[0].product == "" {
		return servers[0].routes()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]string{"status": "ok", "accounts": "isolated"})
	})
	for _, s := range servers {
		prefix := "/products/" + s.product
		mux.Handle(prefix+"/", http.StripPrefix(prefix, s.routes()))
	}
	// Existing provider endpoints continue to verify their original signatures.
	mux.Handle("POST /webhooks/stripe", sharedWebhook(servers, "stripe"))
	mux.Handle("POST /webhooks/wazend", sharedWebhook(servers, "wazend"))
	return mux
}
