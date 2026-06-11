package app

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestNewHTTPServerAppliesConfiguredTimeouts(t *testing.T) {
	cfg := config.HTTP{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 1500 * time.Millisecond,
		WriteTimeout:      3 * time.Second,
		ShutdownTimeout:   2 * time.Second,
	}

	server := NewHTTPServer(cfg, http.NotFoundHandler(), logkit.Noop())

	if server.server.ReadHeaderTimeout != cfg.ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.server.ReadHeaderTimeout, cfg.ReadHeaderTimeout)
	}
	if server.server.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("WriteTimeout = %s, want %s", server.server.WriteTimeout, cfg.WriteTimeout)
	}
	if server.shutdownTimeout != cfg.ShutdownTimeout {
		t.Fatalf("shutdownTimeout = %s, want %s", server.shutdownTimeout, cfg.ShutdownTimeout)
	}
}

func TestHTTPServerServeShutsDownOnContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	cfg := config.HTTP{
		Addr:              listener.Addr().String(),
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		ShutdownTimeout:   time.Second,
	}
	server := NewHTTPServer(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), logkit.Noop())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- server.serve(ctx, listener)
	}()

	assertHTTPStatus(t, "http://"+listener.Addr().String(), http.StatusNoContent)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func assertHTTPStatus(t *testing.T, url string, status int) {
	t.Helper()

	client := http.Client{Timeout: time.Second}
	var lastErr error
	for range 50 {
		resp, err := client.Get(url)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()
			if resp.StatusCode != status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, status)
			}

			return
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("GET %s did not succeed: %v", url, lastErr)
}
