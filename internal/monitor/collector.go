package monitor

import (
	"context"
	"time"

	"einfra/agent/internal/executor"
	"einfra/agent/internal/logger"
	"einfra/agent/internal/transport"
)

// Collector periodically collects and pushes metrics
type Collector struct {
	transport *transport.Client
	registry  *executor.Registry
	interval  time.Duration
}

// NewCollector creates a metric collector
func NewCollector(transport *transport.Client, registry *executor.Registry, interval time.Duration) *Collector {
	return &Collector{
		transport: transport,
		registry:  registry,
		interval:  interval,
	}
}

// Start begins metric collection loop
func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	logger.Info().Msg("Metric collector started")

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Metric collector stopped")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// collect gathers and pushes metrics
func (c *Collector) collect(ctx context.Context) {
	// Execute system_metrics action
	action := &executor.Action{
		ID:   "metric-" + time.Now().Format("20060102150405"),
		Type: "system_metrics",
	}

	result := c.registry.Execute(ctx, action)
	if !result.Success {
		logger.Warn().
			Str("error", result.Error).
			Msg("Failed to collect metrics")
		return
	}

	// Push to backend
	_, err := c.transport.Post(ctx, "/api/v1/agent/metrics", result.Data)
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("Failed to push metrics")
		return
	}

	logger.Debug().Msg("Metrics pushed successfully")
}
