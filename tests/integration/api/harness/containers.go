//go:build integration

package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Containers struct {
	PostgresURL string
	RedisURL    string
	NATSURL     string

	teardown []func(context.Context) error
}

func StartContainers(ctx context.Context) (*Containers, error) {
	c := &Containers{}

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("pingcast_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	c.teardown = append(c.teardown, wrapTerminate(pg))

	pgURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("postgres url: %w", err)
	}
	c.PostgresURL = pgURL

	rd, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("start redis: %w", err)
	}
	c.teardown = append(c.teardown, wrapTerminate(rd))

	rdURL, err := rd.ConnectionString(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("redis url: %w", err)
	}
	c.RedisURL = rdURL

	nats, err := startNATS(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("start nats: %w", err)
	}
	c.teardown = append(c.teardown, wrapTerminate(nats))

	natsHost, err := nats.Host(ctx)
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("nats host: %w", err)
	}
	natsPort, err := nats.MappedPort(ctx, "4222/tcp")
	if err != nil {
		_ = c.Close(ctx)
		return nil, fmt.Errorf("nats port: %w", err)
	}
	c.NATSURL = fmt.Sprintf("nats://%s:%s", natsHost, natsPort.Port())

	return c, nil
}

func (c *Containers) Close(ctx context.Context) error {
	var first error
	for i := len(c.teardown) - 1; i >= 0; i-- {
		if err := c.teardown[i](ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func startNATS(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "nats:2.10-alpine",
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready"),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// RedisAddr strips the redis:// prefix for go-redis clients expecting host:port.
func RedisAddr(url string) string {
	return strings.TrimPrefix(url, "redis://")
}

// wrapTerminate adapts testcontainers.Container.Terminate (variadic options)
// to the teardown func signature without leaking option args.
func wrapTerminate(c testcontainers.Container) func(context.Context) error {
	return func(ctx context.Context) error { return c.Terminate(ctx) }
}
