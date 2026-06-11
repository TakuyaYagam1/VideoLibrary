package redis

import (
	"context"
	"errors"
	"testing"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/wahrwelt-kit/go-cachekit"
)

func TestNewCacheRejectsInvalidConfig(t *testing.T) {
	_, _, err := NewCache(context.Background(), config.Redis{
		Host: "127.0.0.1",
		Port: 0,
	})

	if !errors.Is(err, cachekit.ErrRedisInvalidPort) {
		t.Fatalf("NewCache() error = %v, want ErrRedisInvalidPort", err)
	}
}
