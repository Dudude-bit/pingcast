package checker

import (
	"context"
	"log/slog"
	"sync"

	redisadapter "github.com/kirillinakin/pingcast/internal/adapter/redis"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type CheckHandler func(ctx context.Context, monitor *domain.Monitor)

type WorkerPool struct {
	registry    port.CheckerRegistry
	hostLimiter *redisadapter.HostLimiter
	jobs        chan *domain.Monitor
	handler     CheckHandler
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewWorkerPool(ctx context.Context, workers int, registry port.CheckerRegistry, hostLimiter *redisadapter.HostLimiter, handler CheckHandler) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	wp := &WorkerPool{
		registry:    registry,
		hostLimiter: hostLimiter,
		jobs:        make(chan *domain.Monitor, workers*2),
		handler:     handler,
		ctx:         poolCtx,
		cancel:      cancel,
	}

	for range workers {
		wp.wg.Go(func() {
			wp.worker()
		})
	}

	return wp
}

func (wp *WorkerPool) Submit(m *domain.Monitor) {
	select {
	case wp.jobs <- m:
	case <-wp.ctx.Done():
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}

func (wp *WorkerPool) worker() {
	for {
		select {
		case <-wp.ctx.Done():
			return
		case m := <-wp.jobs:
			if m == nil {
				return
			}

			host, err := wp.registry.Host(m.Type, m.CheckConfig)
			if err != nil {
				slog.Error("failed to resolve host", "monitor_id", m.ID, "error", err)
				continue
			}
			acquired, err := wp.hostLimiter.Acquire(wp.ctx, host)
			if err != nil {
				slog.Error("host limiter acquire failed", "host", host, "error", err)
				continue
			}
			if !acquired {
				slog.Warn("host limit reached, skipping check", "host", host, "monitor_id", m.ID)
				continue
			}

			wp.handler(wp.ctx, m)

			if err := wp.hostLimiter.Release(wp.ctx, host); err != nil {
				slog.Error("host limiter release failed", "host", host, "error", err)
			}
		}
	}
}
