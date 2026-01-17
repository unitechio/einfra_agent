package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"einfra/agent/internal/config"
	"einfra/agent/internal/enroll"
	"einfra/agent/internal/executor"
	"einfra/agent/internal/executor/file"
	package_executor "einfra/agent/internal/executor/package"
	"einfra/agent/internal/executor/service"
	"einfra/agent/internal/executor/system"
	"einfra/agent/internal/executor/user"
	"einfra/agent/internal/identity"
	"einfra/agent/internal/logger"
	"einfra/agent/internal/monitor"
	"einfra/agent/internal/transport"
)

var (
	configPath = flag.String("config", "", "Path to config file")
)

func main() {
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.LogLevel, cfg.LogDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info().Msg("EINFRA Agent starting...")

	// Generate or load identity
	id, err := identity.Generate()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate identity")
	}

	logger.Info().
		Str("node_id", id.NodeID).
		Str("fingerprint", id.Fingerprint).
		Str("hostname", id.Hostname).
		Msg("Agent identity generated")

	// Save NodeID and Fingerprint to config
	cfg.NodeID = id.NodeID
	cfg.Fingerprint = id.Fingerprint

	// Check if enrolled
	enrolled := fileExists(cfg.CertPath) && fileExists(cfg.KeyPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info().Msg("Shutdown signal received")
		cancel()
	}()

	// Enrollment flow
	if !enrolled {
		logger.Info().Msg("Agent not enrolled, starting enrollment...")

		if cfg.EnrollToken == "" {
			logger.Fatal().Msg("EINFRA_ENROLL_TOKEN not set")
		}

		enrollClient := enroll.NewClient(cfg.BackendURL, cfg.EnrollToken, id)

		// Wait for approval
		resp, err := enrollClient.WaitForApproval(ctx, 10*time.Second)
		if err != nil {
			logger.Fatal().Err(err).Msg("Enrollment failed")
		}

		// Generate private key and save certificates
		privateKey, _, err := enroll.GenerateCSR(id)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to generate CSR")
		}

		if err := enroll.SaveCertificates(resp.Certificate, resp.CACert, cfg.CertPath, cfg.CACertPath, cfg.KeyPath, privateKey); err != nil {
			logger.Fatal().Err(err).Msg("Failed to save certificates")
		}

		logger.Info().Msg("Enrollment completed successfully")
	}

	// Initialize transport with mTLS
	transportClient := transport.NewClient(cfg.BackendURL)
	if err := transportClient.EnableMTLS(cfg.CertPath, cfg.KeyPath, cfg.CACertPath); err != nil {
		logger.Fatal().Err(err).Msg("Failed to enable mTLS")
	}

	logger.Info().Msg("mTLS enabled")

	// Initialize executor registry
	registry := executor.NewRegistry()
	registry.Register(service.NewExecutor())
	registry.Register(system.NewExecutor())
	registry.Register(user.NewExecutor())
	registry.Register(file.NewExecutor())
	registry.Register(package_executor.NewExecutor())

	logger.Info().Msg("Executor registry initialized")

	// Start metric collector
	collector := monitor.NewCollector(transportClient, registry, time.Duration(cfg.MetricInterval)*time.Second)
	go collector.Start(ctx)

	// Start heartbeat loop
	go heartbeatLoop(ctx, transportClient, id, time.Duration(cfg.HeartbeatInterval)*time.Second)

	// Start task polling loop
	taskLoop(ctx, transportClient, registry)

	logger.Info().Msg("Agent shutdown complete")
}

// heartbeatLoop sends periodic heartbeats
func heartbeatLoop(ctx context.Context, client *transport.Client, id *identity.Identity, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info().Msg("Heartbeat loop started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := map[string]interface{}{
				"node_id":  id.NodeID,
				"hostname": id.Hostname,
				"status":   "online",
			}

			_, err := client.Post(ctx, "/api/v1/agent/heartbeat", payload)
			if err != nil {
				logger.Warn().Err(err).Msg("Heartbeat failed")
			} else {
				logger.Debug().Msg("Heartbeat sent")
			}
		}
	}
}

// taskLoop polls for and executes tasks
func taskLoop(ctx context.Context, client *transport.Client, registry *executor.Registry) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	logger.Info().Msg("Task polling loop started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Poll for tasks
			resp, err := client.Get(ctx, "/api/v1/agent/tasks/poll")
			if err != nil {
				logger.Debug().Err(err).Msg("Task poll failed")
				continue
			}

			var tasks []executor.Action
			if err := transport.DecodeJSON(resp, &tasks); err != nil {
				logger.Warn().Err(err).Msg("Failed to decode tasks")
				continue
			}

			// Execute tasks
			for _, task := range tasks {
				logger.Info().
					Str("action_id", task.ID).
					Str("action_type", task.Type).
					Msg("Executing task")

				result := registry.Execute(ctx, &task)

				// Report result
				_, err := client.Post(ctx, fmt.Sprintf("/api/v1/agent/tasks/%s/result", task.ID), result)
				if err != nil {
					logger.Warn().
						Err(err).
						Str("action_id", task.ID).
						Msg("Failed to report task result")
				} else {
					logger.Info().
						Str("action_id", task.ID).
						Bool("success", result.Success).
						Msg("Task completed")
				}
			}
		}
	}
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
