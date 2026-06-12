package v1

import (
	"context"

	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

type PingFunc func(context.Context) error

func (f PingFunc) Check(ctx context.Context) error {
	return f(ctx)
}

func NewHealthCheckers(dbPing PingFunc, redisPing PingFunc) map[string]httpkit.Checker {
	return map[string]httpkit.Checker{
		"db":    dbPing,
		"redis": redisPing,
	}
}
