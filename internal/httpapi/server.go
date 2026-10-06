package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/api/idtoken"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type server struct {
	verifyGoogle func(context.Context, string, string) (*idtoken.Payload, error)
	db           *mongo.Database
	secret       []byte
	storage      *storage
}

func Run() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	uri := os.Getenv("MONGODB_URI")
	secret := os.Getenv("JWT_SECRET")
	if uri == "" || len(secret) < 32 {
		log.Fatal("MONGODB_URI and JWT_SECRET (at least 32 bytes) are required")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	dbName := env("MONGODB_DATABASE", "thedate")
	s := &server{db: client.Database(dbName), secret: []byte(secret)}
	s.storage, err = newStorage(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := mongostore.EnsureIndexes(ctx, s.db); err != nil {
		log.Fatal(err)
	}
	if err := mongostore.EnsureTemplates(ctx, s.db); err != nil {
		log.Fatal(err)
	}
	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); s.deploymentWorker(runCtx) }()
	port := env("PORT", "8080")
	log.Printf("The Date API listening on :%s", port)
	httpServer := &http.Server{Addr: ":" + port, Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-runCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print("HTTP server stopped unexpectedly")
		stop()
	}
	stop()
	<-workerDone
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeCancel()
	_ = client.Disconnect(closeCtx)
}

func env(k, fallback string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return fallback
}
