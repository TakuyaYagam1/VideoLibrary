//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	postgresImage = "postgres:18.4-trixie"
	redisImage    = "redis:8.8.0-trixie"
)

func startPostgresContainer(t *testing.T, ctx context.Context) config.PostgreSQL {
	t.Helper()

	container, err := tcpostgres.Run(ctx,
		postgresImage,
		tcpostgres.WithDatabase("videolibrary"),
		tcpostgres.WithUsername("videolibrary"),
		tcpostgres.WithPassword("videolibrary"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if terminateErr := testcontainers.TerminateContainer(container); terminateErr != nil {
			t.Fatalf("terminate postgres container: %v", terminateErr)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	return config.PostgreSQL{
		DSN:            dsn,
		MaxConns:       4,
		MinConns:       1,
		RetryTimeout:   time.Second,
		ConnectTimeout: 5 * time.Second,
		MigrationsPath: "../migrations",
	}
}

func startRedisContainer(t *testing.T, ctx context.Context) config.Redis {
	t.Helper()

	container, err := tcredis.Run(ctx, redisImage)
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		if terminateErr := testcontainers.TerminateContainer(container); terminateErr != nil {
			t.Fatalf("terminate redis container: %v", terminateErr)
		}
	})

	connectionString, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}

	host, port, err := redisEndpoint(connectionString)
	if err != nil {
		t.Fatalf("parse redis connection string: %v", err)
	}

	return config.Redis{
		Host:         host,
		Port:         port,
		DB:           0,
		PoolSize:     4,
		MinIdleConns: 1,
	}
}

func redisEndpoint(connectionString string) (string, int, error) {
	parsed, err := url.Parse(connectionString)
	if err != nil {
		return "", 0, err
	}

	host, portString, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", 0, fmt.Errorf("parse port %q: %w", portString, err)
	}

	return host, port, nil
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free tcp addr: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Fatalf("close free tcp listener: %v", err)
		}
	}()

	return listener.Addr().String()
}
