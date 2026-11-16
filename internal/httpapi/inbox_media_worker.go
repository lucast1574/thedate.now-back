package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lucast1574/thedate.now-back/internal/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *server) refreshMessageMedia(ctx context.Context, m core.InboxMessage) {
	if m.ProviderID == "" || m.Media == nil || m.Media.URL != "" {
		return
	}
	config := wazendFor(m.Media.BucketKind)
	if !config.ready() {
		return
	}
	base, err := url.Parse(config.Base)
	if err != nil || base.Scheme != "https" {
		return
	}
	endpoint := strings.TrimSuffix(config.Base, "/") + "/api/" + url.PathEscape(m.Session) + "/chats/" + url.PathEscape(strings.TrimPrefix(m.Phone, "+")+"@c.us") + "/messages/" + url.PathEscape(m.ProviderID) + "?downloadMedia=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Api-Key", config.Key)
	res, err := wazendClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return
	}
	var message struct {
		Media *struct {
			URL string `json:"url"`
		} `json:"media"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&message) != nil || message.Media == nil || !safeMessageMediaURL(config.Base, message.Media.URL) {
		return
	}
	s.db.Collection("inboxMessages").UpdateOne(ctx, bson.M{"_id": m.ID, "media.available": false}, bson.M{"$set": bson.M{"media.url": message.Media.URL}})
}
func (s *server) inboxMediaWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.storage == nil {
				continue
			}
			attempt, cancel := context.WithTimeout(ctx, 45*time.Second)
			cur, err := s.db.Collection("inboxMessages").Find(attempt, bson.M{"media.available": false, "receivedAt": bson.M{"$gte": time.Now().Add(-7 * 24 * time.Hour)}}, options.Find().SetLimit(5).SetSort(bson.D{{Key: "mediaCheckedAt", Value: 1}}))
			if err == nil {
				for cur.Next(attempt) {
					var m core.InboxMessage
					if cur.Decode(&m) != nil {
						continue
					}
					s.refreshMessageMedia(attempt, m)
					if m.Media != nil && m.Media.URL == "" {
						s.db.Collection("inboxMessages").FindOne(attempt, bson.M{"_id": m.ID}).Decode(&m)
					}
					_ = s.ensureMessageMedia(attempt, m)
					s.db.Collection("inboxMessages").UpdateOne(attempt, bson.M{"_id": m.ID}, bson.M{"$set": bson.M{"mediaCheckedAt": time.Now().UTC()}})
				}
				cur.Close(attempt)
			}
			cancel()
		}
	}
}
