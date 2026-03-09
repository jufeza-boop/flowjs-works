// Package main is the entry point for the flowjs-works orchestrator service.
// The orchestrator listens for lifecycle events (deploy/stop) on NATS and
// manages dedicated Kubernetes Deployments for each deployed flow, providing
// per-flow isolation as described in Phase 7 (Task 7.3).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"flowjs-works/orchestrator/internal/api"
	"flowjs-works/orchestrator/internal/controller"
)

func main() {
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	httpAddr := envOrDefault("HTTP_ADDR", ":8081")
	namespace := envOrDefault("K8S_NAMESPACE", "flowjs")
	engineImage := envOrDefault("ENGINE_IMAGE", "flowjs-engine:latest")
	databaseURL := envOrDefault("DATABASE_URL", "")
	aesKey := envOrDefault("SECRETS_AES_KEY", "")

	cfg := controller.Config{
		Namespace:   namespace,
		EngineImage: engineImage,
		NATSUrl:     natsURL,
		DatabaseURL: databaseURL,
		AESKey:      aesKey,
	}

	ctrl, err := controller.New(cfg)
	if err != nil {
		log.Fatalf("orchestrator: init controller: %v", err)
	}

	srv, err := api.New(ctrl, natsURL, httpAddr)
	if err != nil {
		log.Fatalf("orchestrator: init server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Println("orchestrator: shutting down")
		cancel()
	}()

	if err := srv.Start(ctx); err != nil {
		log.Printf("orchestrator: server stopped: %v", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
