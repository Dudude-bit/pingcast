package checker

import (
	"context"
	"log/slog"
	"net/url"
	"sync"
)

type CheckHandler func(ctx context.Context, monitor *MonitorInfo, result *CheckResult)

type WorkerPool struct {
	client      *Client
	hostLimiter *HostLimiter
	jobs        chan *MonitorInfo
	handler     CheckHandler
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewWorkerPool(ctx context.Context, workers int, client *Client, handler CheckHandler) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	wp := &WorkerPool{
		client:      client,
		hostLimiter: NewHostLimiter(),
		jobs:        make(chan *MonitorInfo, workers*2),
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

func (wp *WorkerPool) Submit(m *MonitorInfo) {
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

			host := extractHost(m.URL)
			wp.hostLimiter.Acquire(host)
			result := wp.client.Check(wp.ctx, m)
			wp.hostLimiter.Release(host)

			slog.Info("check completed",
				"monitor_id", m.ID,
				"url", m.URL,
				"status", result.Status,
				"response_time_ms", result.ResponseTimeMs,
			)

			wp.handler(wp.ctx, m, result)
		}
	}
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
