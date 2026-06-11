package config

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestLoadReadsConfigFromEnvironment(t *testing.T) {
	env := map[string]string{
		"APP_NAME":                 "videolibrary",
		"APP_ENV":                  "test",
		"HTTP_ADDR":                "127.0.0.1:8080",
		"HTTP_READ_HEADER_TIMEOUT": "1500ms",
		"HTTP_WRITE_TIMEOUT":       "4s",
		"HTTP_SHUTDOWN_TIMEOUT":    "5s",
		"POSTGRES_DSN":             "postgres://videolibrary:videolibrary@127.0.0.1:5432/videolibrary?sslmode=disable",
		"POSTGRES_MAX_CONNS":       "12",
		"POSTGRES_MIN_CONNS":       "2",
		"POSTGRES_RETRY_TIMEOUT":   "4s",
		"POSTGRES_CONNECT_TIMEOUT": "2s",
		"POSTGRES_MIGRATIONS_PATH": "migrations",
		"REDIS_HOST":               "127.0.0.1",
		"REDIS_PORT":               "6379",
		"REDIS_PASSWORD":           "redis-password",
		"REDIS_DB":                 "2",
		"REDIS_POOL_SIZE":          "16",
		"REDIS_MIN_IDLE_CONNS":     "4",
		"CACHE_VIDEO_LIST_TTL":     "2m",
		"HEALTH_CHECK_TIMEOUT":     "3s",
		"SEAWEEDFS_PUBLIC_URL":     "http://127.0.0.1:8888",
		"LOG_LEVEL":                "debug",
		"LOG_FORMAT":               "json",
	}

	cfg, err := LoadFromLookup(mapLookup(env))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "videolibrary" {
		t.Fatalf("App.Name = %q", cfg.App.Name)
	}
	if cfg.HTTP.Addr != "127.0.0.1:8080" {
		t.Fatalf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadHeaderTimeout != 1500*time.Millisecond {
		t.Fatalf("HTTP.ReadHeaderTimeout = %s", cfg.HTTP.ReadHeaderTimeout)
	}
	if cfg.HTTP.WriteTimeout != 4*time.Second {
		t.Fatalf("HTTP.WriteTimeout = %s", cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.ShutdownTimeout != 5*time.Second {
		t.Fatalf("HTTP.ShutdownTimeout = %s", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.PostgreSQL.DSN == "" {
		t.Fatal("PostgreSQL.DSN is empty")
	}
	if cfg.PostgreSQL.MaxConns != 12 {
		t.Fatalf("PostgreSQL.MaxConns = %d", cfg.PostgreSQL.MaxConns)
	}
	if cfg.PostgreSQL.MinConns != 2 {
		t.Fatalf("PostgreSQL.MinConns = %d", cfg.PostgreSQL.MinConns)
	}
	if cfg.PostgreSQL.RetryTimeout != 4*time.Second {
		t.Fatalf("PostgreSQL.RetryTimeout = %s", cfg.PostgreSQL.RetryTimeout)
	}
	if cfg.PostgreSQL.ConnectTimeout != 2*time.Second {
		t.Fatalf("PostgreSQL.ConnectTimeout = %s", cfg.PostgreSQL.ConnectTimeout)
	}
	if cfg.PostgreSQL.MigrationsPath != "migrations" {
		t.Fatalf("PostgreSQL.MigrationsPath = %q", cfg.PostgreSQL.MigrationsPath)
	}
	if cfg.Redis.Host != "127.0.0.1" {
		t.Fatalf("Redis.Host = %q", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Fatalf("Redis.Port = %d", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "redis-password" {
		t.Fatalf("Redis.Password = %q", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d", cfg.Redis.DB)
	}
	if cfg.Redis.PoolSize != 16 {
		t.Fatalf("Redis.PoolSize = %d", cfg.Redis.PoolSize)
	}
	if cfg.Redis.MinIdleConns != 4 {
		t.Fatalf("Redis.MinIdleConns = %d", cfg.Redis.MinIdleConns)
	}
	if cfg.Cache.VideoListTTL != 2*time.Minute {
		t.Fatalf("Cache.VideoListTTL = %s", cfg.Cache.VideoListTTL)
	}
	if cfg.Health.CheckTimeout != 3*time.Second {
		t.Fatalf("Health.CheckTimeout = %s", cfg.Health.CheckTimeout)
	}
	if cfg.SeaweedFS.PublicURL != "http://127.0.0.1:8888" {
		t.Fatalf("SeaweedFS.PublicURL = %q", cfg.SeaweedFS.PublicURL)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q", cfg.Log.Level)
	}
	if cfg.Log.Output != "console" {
		t.Fatalf("Log.Output = %q", cfg.Log.Output)
	}
}

func TestLoadReportsMissingRequiredEnvironment(t *testing.T) {
	env := map[string]string{
		"APP_NAME":  "videolibrary",
		"HTTP_ADDR": "127.0.0.1:8080",
	}

	_, err := LoadFromLookup(mapLookup(env))
	if err == nil {
		t.Fatal("Load() error = nil")
	}

	var missing MissingRequiredEnvError
	if !errors.As(err, &missing) {
		t.Fatalf("Load() error type = %T", err)
	}

	expected := []string{
		"APP_ENV",
		"LOG_FORMAT",
		"LOG_LEVEL",
		"POSTGRES_DSN",
		"REDIS_HOST",
		"REDIS_PORT",
		"SEAWEEDFS_PUBLIC_URL",
	}
	if !reflect.DeepEqual(missing.Names, expected) {
		t.Fatalf("missing names = %#v, want %#v", missing.Names, expected)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	env := map[string]string{
		"APP_NAME":             "videolibrary",
		"APP_ENV":              "test",
		"HTTP_ADDR":            "127.0.0.1:8080",
		"POSTGRES_DSN":         "postgres://videolibrary:videolibrary@127.0.0.1:5432/videolibrary?sslmode=disable",
		"REDIS_HOST":           "127.0.0.1",
		"REDIS_PORT":           "6379",
		"SEAWEEDFS_PUBLIC_URL": "http://127.0.0.1:8888",
		"LOG_LEVEL":            "verbose",
		"LOG_FORMAT":           "json",
	}

	if _, err := LoadFromLookup(mapLookup(env)); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadRejectsInvalidPostgresDuration(t *testing.T) {
	env := map[string]string{
		"APP_NAME":               "videolibrary",
		"APP_ENV":                "test",
		"HTTP_ADDR":              "127.0.0.1:8080",
		"POSTGRES_DSN":           "postgres://videolibrary:videolibrary@127.0.0.1:5432/videolibrary?sslmode=disable",
		"POSTGRES_RETRY_TIMEOUT": "soon",
		"REDIS_HOST":             "127.0.0.1",
		"REDIS_PORT":             "6379",
		"SEAWEEDFS_PUBLIC_URL":   "http://127.0.0.1:8888",
		"LOG_LEVEL":              "debug",
		"LOG_FORMAT":             "json",
	}

	if _, err := LoadFromLookup(mapLookup(env)); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadSupportsLegacyRedisAddr(t *testing.T) {
	env := map[string]string{
		"APP_NAME":             "videolibrary",
		"APP_ENV":              "test",
		"HTTP_ADDR":            "127.0.0.1:8080",
		"POSTGRES_DSN":         "postgres://videolibrary:videolibrary@127.0.0.1:5432/videolibrary?sslmode=disable",
		"REDIS_ADDR":           "127.0.0.1:6379",
		"SEAWEEDFS_PUBLIC_URL": "http://127.0.0.1:8888",
		"LOG_LEVEL":            "debug",
		"LOG_FORMAT":           "json",
	}

	cfg, err := LoadFromLookup(mapLookup(env))
	if err != nil {
		t.Fatalf("LoadFromLookup() error = %v", err)
	}
	if cfg.Redis.Host != "127.0.0.1" {
		t.Fatalf("Redis.Host = %q", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Fatalf("Redis.Port = %d", cfg.Redis.Port)
	}
}

func TestLoadRejectsFileOutputWithoutPath(t *testing.T) {
	env := map[string]string{
		"APP_NAME":             "videolibrary",
		"APP_ENV":              "test",
		"HTTP_ADDR":            "127.0.0.1:8080",
		"POSTGRES_DSN":         "postgres://videolibrary:videolibrary@127.0.0.1:5432/videolibrary?sslmode=disable",
		"REDIS_HOST":           "127.0.0.1",
		"REDIS_PORT":           "6379",
		"SEAWEEDFS_PUBLIC_URL": "http://127.0.0.1:8888",
		"LOG_LEVEL":            "debug",
		"LOG_FORMAT":           "json",
		"LOG_OUTPUT":           "file",
	}

	if _, err := LoadFromLookup(mapLookup(env)); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func mapLookup(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}
