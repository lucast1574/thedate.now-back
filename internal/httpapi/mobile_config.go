package httpapi

import (
	"context"
	"github.com/lucast1574/thedate.now-back/internal/mobilepush"
	"os"
)

func (s *server) configureMobile(ctx context.Context) error {
	key, project := "FCM_GENERAL_CREDENTIALS", "the-date-5c49a"
	if s.product == "wedding" {
		key, project = "FCM_WEDDING_CREDENTIALS", "save-the-date-b7670"
	}
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	client, err := mobilepush.New(ctx, []byte(raw), project)
	if err != nil {
		return err
	}
	s.mobileSender = client
	return nil
}
