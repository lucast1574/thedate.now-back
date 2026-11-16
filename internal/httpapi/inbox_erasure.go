package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"github.com/minio/minio-go/v7"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *server) eraseInboxMedia(ctx context.Context, e core.Event) error {
	if s.storage == nil {
		return nil
	}
	cur, err := s.db.Collection("inboxMessages").Find(ctx, bson.M{"eventId": e.ID, "media.available": true})
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var m core.InboxMessage
		if err = cur.Decode(&m); err != nil {
			return err
		}
		if m.Media != nil && m.Media.Key != "" {
			if err = s.storage.client.RemoveObject(ctx, s.storage.bucket(m.Media.BucketKind), m.Media.Key, minio.RemoveObjectOptions{}); err != nil {
				return err
			}
		}
	}
	return cur.Err()
}
