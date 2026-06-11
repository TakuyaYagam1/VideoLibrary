package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestNewLoggerStoresLoggerInContext(t *testing.T) {
	logger, err := NewLogger(testConfig())
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	ctx := logkit.IntoContext(context.Background(), logger)
	if got := logkit.FromContext(ctx); got != logger {
		t.Fatal("FromContext() did not return configured logger")
	}
}

func TestNewLoggerSupportsFileOutput(t *testing.T) {
	cfg := testConfig()
	cfg.Log.Output = "file"
	cfg.Log.FilePath = filepath.Join(t.TempDir(), "app.log")

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info("file output test")
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(cfg.Log.FilePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "file output test") {
		t.Fatalf("log file does not contain expected message: %s", data)
	}
}

func TestRunAcceptsNoopLoggerFromContext(t *testing.T) {
	ctx := logkit.IntoContext(context.Background(), logkit.Noop())

	if err := New(testConfig()).Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func testConfig() config.Config {
	return config.Config{
		App: config.App{
			Name: "videolibrary",
			Env:  "test",
		},
		HTTP: config.HTTP{
			Addr: "127.0.0.1:8080",
		},
		Log: config.Log{
			Level:  "debug",
			Format: "json",
			Output: "console",
		},
	}
}
