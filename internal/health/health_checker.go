package health

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type HealthChecker struct {
	timeout    time.Duration
	pauseMutex sync.Mutex
	pauseCond  *sync.Cond
	IsIll      bool
	ctx        context.Context
	log        zerolog.Logger
	pingFn     func(context.Context) error
}

func NewHealthChecker(
	ctx context.Context,
	t time.Duration,
	pfn func(context.Context) error,
	l zerolog.Logger,
) *HealthChecker {
	h := HealthChecker{
		timeout: t,
		ctx:     ctx,
		pingFn:  pfn,
		log:     l,
	}

	h.pauseCond = sync.NewCond(&h.pauseMutex)
	go h.pingConnection(h.ctx)
	return &h // escapes to heap :(
}

func (h *HealthChecker) pingConnection(ctx context.Context) {
	pingsErr := make(chan error)
	ticker := time.NewTicker(h.timeout)
	defer ticker.Stop()

	for {
		ctxPing, cancelPing := context.WithTimeout(ctx, time.Second)

		go func() {
			pingsErr <- h.pingFn(ctxPing)
		}()

		select {
		case <-ctx.Done():
			cancelPing()
			ticker.Stop()
			h.log.Warn().Str("message", "finishing health checker").Send()
			return
		case <-ctxPing.Done():
			h.setHealth(true)
			<-ticker.C
			h.log.Error().Err(ctxPing.Err()).Send()
			continue
		case err := <-pingsErr:
			if err != nil {
				h.setHealth(true)
				h.log.Error().Err(err).Str("message", "error on ping").Send()
				<-ticker.C
				continue
			}
		}

		h.setHealth(false)
		<-ticker.C
	}
}

func (h *HealthChecker) setHealth(isIll bool) {
	h.pauseMutex.Lock()
	defer h.pauseMutex.Unlock()

	if h.IsIll != isIll {
		h.IsIll = isIll
		if h.IsIll {
			h.log.Warn().Str("message", "connection is ill, pausing all incoming requests").Send()
		} else {
			h.log.Warn().Str("message", "connection is healthy, resuming all incoming requests").Send()
		}
		h.pauseCond.Broadcast()
	}
}

func (h *HealthChecker) WaitForHealing() {
	h.pauseMutex.Lock()
	for h.IsIll {
		h.log.Warn().Str("message", "waiting for healing").Send()
		h.pauseCond.Wait()
	}
	h.pauseMutex.Unlock()
}
