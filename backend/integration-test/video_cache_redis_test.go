//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	redisconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/redis"
	"github.com/google/uuid"
	"github.com/wahrwelt-kit/go-cachekit"
)

type redisCacheProbe struct {
	Message string
	Count   int
}

func TestRedisCacheIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cache, err := redisconnector.NewCache(ctx, startRedisContainer(t, ctx))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	key := "videolibrary:test:redis-cache:" + uuid.NewString()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if delErr := cache.Del(cleanupCtx, key); delErr != nil {
			t.Fatalf("cache.Del() cleanup error = %v", delErr)
		}
	})

	want := redisCacheProbe{Message: "ok", Count: 2}
	if setErr := cache.Set(ctx, key, want, time.Minute); setErr != nil {
		t.Fatalf("cache.Set() error = %v", setErr)
	}

	loadCalled := false
	got, err := cachekit.GetOrLoad(cache, ctx, key, time.Minute, func(context.Context) (redisCacheProbe, error) {
		loadCalled = true
		return redisCacheProbe{}, errors.New("cache hit should not load")
	})
	if err != nil {
		t.Fatalf("GetOrLoad() error = %v", err)
	}
	if got != want {
		t.Fatalf("GetOrLoad() = %#v, want %#v", got, want)
	}
	if loadCalled {
		t.Fatal("GetOrLoad() called load function on cache hit")
	}
}
