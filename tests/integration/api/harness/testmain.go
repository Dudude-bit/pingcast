//go:build integration

package harness

import (
	"context"
	"log/slog"
	"time"
)

var global *globalState

type globalState struct {
	containers *Containers
}

// Setup boots containers. Idempotent — second call is a no-op.
func Setup() error {
	if global != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c, err := StartContainers(ctx)
	if err != nil {
		return err
	}
	global = &globalState{containers: c}
	return nil
}

// Teardown releases containers. Safe to call if Setup was never called.
func Teardown() {
	if global == nil || global.containers == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := global.containers.Close(ctx); err != nil {
		slog.Error("harness teardown", "error", err)
	}
	global = nil
}

// GetContainers returns the shared container URLs. Nil before Setup.
func GetContainers() *Containers {
	if global == nil {
		return nil
	}
	return global.containers
}
