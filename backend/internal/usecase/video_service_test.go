package usecase

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	usecasemock "github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase/mock"
	"github.com/google/uuid"
	testifymock "github.com/stretchr/testify/mock"
)

func TestNewVideoServiceValidation(t *testing.T) {
	tests := []struct {
		name     string
		repoNil  bool
		cacheNil bool
		ttl      time.Duration
		wantErr  error
	}{
		{
			name:    "repository required",
			repoNil: true,
			ttl:     time.Minute,
			wantErr: errVideoRepositoryRequired,
		},
		{
			name:     "cache required",
			cacheNil: true,
			ttl:      time.Minute,
			wantErr:  errVideoCacheRequired,
		},
		{
			name:    "ttl required",
			ttl:     0,
			wantErr: errVideoListTTLRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repository VideoRepository = usecasemock.NewMockVideoRepository(t)
			if tt.repoNil {
				repository = nil
			}
			var cache VideoCache = usecasemock.NewMockVideoCache(t)
			if tt.cacheNil {
				cache = nil
			}

			_, err := NewVideoService(repository, cache, tt.ttl)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewVideoService() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVideoServiceListVideos(t *testing.T) {
	ctx := context.Background()
	ttl := time.Minute
	cachedVideos := []domain.Video{testVideo("cache-hit", 1)}
	loadedVideos := []domain.Video{testVideo("cache-miss", 2)}
	errCacheUnavailable := errors.New("cache unavailable")
	errRepositoryUnavailable := errors.New("repository unavailable")

	tests := []struct {
		name    string
		setup   func(*usecasemock.MockVideoRepository, *usecasemock.MockVideoCache)
		want    []domain.Video
		wantErr error
	}{
		{
			name: "cache hit returns cached videos",
			setup: func(_ *usecasemock.MockVideoRepository, cache *usecasemock.MockVideoCache) {
				cache.EXPECT().
					GetOrLoadVideos(ctx, VideoListCacheKey, ttl, testifymock.Anything).
					Return(cachedVideos, nil).
					Once()
			},
			want: cachedVideos,
		},
		{
			name: "cache miss loads videos once through repository",
			setup: func(repository *usecasemock.MockVideoRepository, cache *usecasemock.MockVideoCache) {
				repository.EXPECT().
					ListVideos(testifymock.Anything).
					Return(loadedVideos, nil).
					Once()
				cache.EXPECT().
					GetOrLoadVideos(ctx, VideoListCacheKey, ttl, testifymock.Anything).
					RunAndReturn(func(loadCtx context.Context, _ string, _ time.Duration, loadFn func(context.Context) ([]domain.Video, error)) ([]domain.Video, error) {
						return loadFn(loadCtx)
					}).
					Once()
			},
			want: loadedVideos,
		},
		{
			name: "cache error is wrapped",
			setup: func(_ *usecasemock.MockVideoRepository, cache *usecasemock.MockVideoCache) {
				cache.EXPECT().
					GetOrLoadVideos(ctx, VideoListCacheKey, ttl, testifymock.Anything).
					Return(nil, errCacheUnavailable).
					Once()
			},
			wantErr: errCacheUnavailable,
		},
		{
			name: "repository load error is wrapped",
			setup: func(repository *usecasemock.MockVideoRepository, cache *usecasemock.MockVideoCache) {
				repository.EXPECT().
					ListVideos(testifymock.Anything).
					Return(nil, errRepositoryUnavailable).
					Once()
				cache.EXPECT().
					GetOrLoadVideos(ctx, VideoListCacheKey, ttl, testifymock.Anything).
					RunAndReturn(func(loadCtx context.Context, _ string, _ time.Duration, loadFn func(context.Context) ([]domain.Video, error)) ([]domain.Video, error) {
						return loadFn(loadCtx)
					}).
					Once()
			},
			wantErr: errRepositoryUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := usecasemock.NewMockVideoRepository(t)
			cache := usecasemock.NewMockVideoCache(t)
			tt.setup(repository, cache)
			service := newTestVideoService(t, repository, cache, ttl)

			got, err := service.ListVideos(ctx)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ListVideos() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListVideos() error = %v", err)
			}
			requireVideos(t, got, tt.want)
		})
	}
}

func TestVideoServiceListVideosSingleflightMiss(t *testing.T) {
	ctx := context.Background()
	ttl := time.Minute
	want := []domain.Video{testVideo("singleflight", 3)}
	repository := usecasemock.NewMockVideoRepository(t)
	cache := usecasemock.NewMockVideoCache(t)
	cacheEntered := make(chan struct{})
	releaseLoad := make(chan struct{})
	var cacheCalls atomic.Int64
	var repositoryCalls atomic.Int64

	repository.EXPECT().
		ListVideos(testifymock.Anything).
		RunAndReturn(func(context.Context) ([]domain.Video, error) {
			repositoryCalls.Add(1)
			return want, nil
		}).
		Once()
	cache.EXPECT().
		GetOrLoadVideos(ctx, VideoListCacheKey, ttl, testifymock.Anything).
		RunAndReturn(func(loadCtx context.Context, _ string, _ time.Duration, loadFn func(context.Context) ([]domain.Video, error)) ([]domain.Video, error) {
			cacheCalls.Add(1)
			close(cacheEntered)
			<-releaseLoad
			return loadFn(loadCtx)
		}).
		Once()
	service := newTestVideoService(t, repository, cache, ttl)

	const goroutines = 8
	start := make(chan struct{})
	var started atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			started.Add(1)
			got, err := service.ListVideos(ctx)
			if err != nil {
				errs <- err
				return
			}
			if !reflect.DeepEqual(got, want) {
				errs <- errors.New("unexpected videos")
				return
			}
			errs <- nil
		}()
	}

	close(start)
	waitForAtomic(t, &started, goroutines)
	waitForChannel(t, cacheEntered, "cache load did not start")
	close(releaseLoad)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("ListVideos() goroutine error = %v", err)
		}
	}
	if got := cacheCalls.Load(); got != 1 {
		t.Fatalf("cache GetOrLoadVideos calls = %d, want 1", got)
	}
	if got := repositoryCalls.Load(); got != 1 {
		t.Fatalf("repository ListVideos calls = %d, want 1", got)
	}
}

func TestVideoServiceIncrementViews(t *testing.T) {
	ctx := context.Background()
	ttl := time.Minute
	videoID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e544444")
	incremented := testVideo("increment", 8)
	incremented.ID = videoID
	errStorageUnavailable := errors.New("storage unavailable")

	tests := []struct {
		name    string
		setup   func(*usecasemock.MockVideoRepository, *usecasemock.MockVideoCache)
		want    domain.Video
		wantErr error
	}{
		{
			name: "uses repository outbox path",
			setup: func(repository *usecasemock.MockVideoRepository, _ *usecasemock.MockVideoCache) {
				repository.EXPECT().
					IncrementViewsWithOutbox(ctx, videoID).
					Return(incremented, nil).
					Once()
			},
			want: incremented,
		},
		{
			name: "preserves not found",
			setup: func(repository *usecasemock.MockVideoRepository, _ *usecasemock.MockVideoCache) {
				repository.EXPECT().
					IncrementViewsWithOutbox(ctx, videoID).
					Return(domain.Video{}, domain.ErrVideoNotFound).
					Once()
			},
			wantErr: domain.ErrVideoNotFound,
		},
		{
			name: "wraps repository error",
			setup: func(repository *usecasemock.MockVideoRepository, _ *usecasemock.MockVideoCache) {
				repository.EXPECT().
					IncrementViewsWithOutbox(ctx, videoID).
					Return(domain.Video{}, errStorageUnavailable).
					Once()
			},
			wantErr: errStorageUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := usecasemock.NewMockVideoRepository(t)
			cache := usecasemock.NewMockVideoCache(t)
			tt.setup(repository, cache)
			service := newTestVideoService(t, repository, cache, ttl)

			got, err := service.IncrementViews(ctx, videoID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("IncrementViews() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("IncrementViews() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("IncrementViews() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
