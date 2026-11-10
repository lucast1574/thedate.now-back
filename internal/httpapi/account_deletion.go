package httpapi

import (
	"context"
	"errors"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/lucast1574/thedate.now-back/internal/mongostore"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"log"
	"net/http"
	"time"
)

func (s *server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	u, err := s.user(r)
	if err != nil {
		bad(w, 401, "Inicia sesión para eliminar tu cuenta.")
		return
	}
	repo := mongostore.AccountDeletion{DB: s.db}
	if err = repo.Request(r.Context(), u.ID); err != nil {
		bad(w, 500, "No se pudo eliminar tu cuenta. Vuelve a intentarlo.")
		return
	}
	// The persisted intent survives browser disconnects and provider outages.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = s.processAccountDeletion(ctx, u.ID); err != nil {
		log.Print("Account deletion pending; worker will resume")
		reply(w, http.StatusAccepted, map[string]string{"status": "deleting"})
		return
	}
	reply(w, 200, map[string]string{"status": "deleted"})
}

func (s *server) processAccountDeletion(ctx context.Context, id string) error {
	repo := mongostore.AccountDeletion{DB: s.db}
	u, claim, err := repo.Claim(ctx, id)
	if err != nil {
		return err
	}
	defer func() {
		release, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		repo.Release(release, u.ID, claim)
	}()
	events, err := repo.Events(ctx, u.ID)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err = s.eraseAccountEvent(ctx, e); err != nil {
			return err
		}
		if err = repo.RemoveEvent(ctx, e); err != nil {
			return err
		}
	}
	return repo.Finish(ctx, u, claim)
}

func (s *server) eraseAccountEvent(ctx context.Context, e core.Event) error {
	repo := mongostore.Deployments{DB: s.db}
	d, err := repo.Get(ctx, e.ID)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	if err == nil {
		// Honor publication leases so an in-flight deployment cannot republish after deletion.
		var active bson.M
		err = s.db.Collection("deployments").FindOne(ctx, bson.M{"_id": e.ID, "leaseUntil": bson.M{"$gt": time.Now().UTC()}}).Decode(&active)
		if err == nil {
			return errors.New("publication still in progress")
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		locked, claim, lockErr := repo.ClaimClosure(ctx, e.ID, time.Now().UTC())
		if lockErr != nil && d.Phase != "stopped" {
			return lockErr
		}
		if lockErr == nil {
			if locked.ApplicationID != "" {
				provider, providerErr := deploymentClientFor(s.product)
				if providerErr != nil {
					return providerErr
				}
				if err = provider.StopInvitation(ctx, locked); err != nil {
					return err
				}
			}
			locked.Phase, locked.Error = "stopped", ""
			if err = repo.Save(ctx, locked, claim); err != nil {
				return err
			}
			if err = repo.Finish(ctx, locked, claim); err != nil {
				return err
			}
		}
	}
	if s.storage != nil {
		keys := append(append([]string{}, e.PhotoKeys...), e.AudioKeys...)
		for _, key := range keys {
			if err = s.storage.client.RemoveObject(ctx, s.storage.bucket(e.Kind), key, minio.RemoveObjectOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *server) accountDeletionWorker(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
		err := s.processAccountDeletion(attempt, "")
		cancel()
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) && ctx.Err() == nil {
			log.Print("Account deletion will retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
