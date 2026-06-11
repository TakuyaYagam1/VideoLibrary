package restapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

func TestHandlerGetHealthzOK(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(NewHealthCheckers(
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	), time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
}

func TestHandlerGetHealthzDegraded(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(NewHealthCheckers(
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("redis unavailable") },
	), time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusServiceUnavailable, openapi.Degraded)
}

func TestHandlerGetHealthzUsesTimeout(t *testing.T) {
	const timeout = 25 * time.Millisecond
	var deadline time.Time
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(map[string]httpkit.Checker{
		"db": checkerFunc(func(ctx context.Context) error {
			var ok bool
			deadline, ok = ctx.Deadline()
			if !ok {
				return errors.New("missing deadline")
			}

			return nil
		}),
	}, timeout)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	startedAt := time.Now()

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
	if deadline.Before(startedAt) || deadline.After(startedAt.Add(timeout+25*time.Millisecond)) {
		t.Fatalf("deadline = %s, want within configured timeout from %s", deadline, startedAt)
	}
}

func TestHandlerGetHealthzRunsChecksInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(map[string]httpkit.Checker{
		"db": checkerFunc(func(context.Context) error {
			started <- "db"
			<-release
			return nil
		}),
		"redis": checkerFunc(func(context.Context) error {
			started <- "redis"
			<-release
			return nil
		}),
	}, time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()

	waitForStartedChecks(t, started, 2)
	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health handler did not finish")
	}

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
}
