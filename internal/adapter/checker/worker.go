package checker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

type CheckHandler func(ctx context.Context, monitor *domain.Monitor)

type WorkerPool struct {
	registry    port.CheckerRegistry
	hostLimiter *HostLimiter
	jobs        chan *domain.Monitor
	handler     CheckHandler
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewWorkerPool(ctx context.Context, workers int, registry port.CheckerRegistry, handler CheckHandler) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	wp := &WorkerPool{
		registry:    registry,
		hostLimiter: NewHostLimiter(),
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

			host := wp.registry.Host(m.Type, m.CheckConfig)
			wp.hostLimiter.Acquire(host)
			wp.handler(wp.ctx, m)
			wp.hostLimiter.Release(host)

			slog.Info("check dispatched",
				"monitor_id", m.ID,
				"type", m.Type,
			)
		}
	}
}
