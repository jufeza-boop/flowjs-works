// Package api provides the HTTP API and NATS subscriber for the orchestrator service.
// It listens for lifecycle events published by the engine on NATS and translates
// them into per-flow Kubernetes Deployment create/delete calls.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	nats "github.com/nats-io/nats.go"

	"flowjs-works/orchestrator/internal/controller"
)

const lifecycleSubject = "audit.logs"

// LifecycleEvent matches the shape emitted by engine.SendLifecycleAuditLog.
type LifecycleEvent struct {
	FlowID   string                 `json:"flow_id"`
	NodeType string                 `json:"node_type"`
	Status   string                 `json:"status"`
	Input    map[string]interface{} `json:"input"`
}

// Server wires together the NATS subscriber and the HTTP status API.
type Server struct {
	ctrl     *controller.FlowController
	conn     *nats.Conn
	httpAddr string
}

// New creates a Server. Call Start to begin handling events.
func New(ctrl *controller.FlowController, natsURL, httpAddr string) (*Server, error) {
	nc, err := nats.Connect(natsURL,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err
	}
	return &Server{ctrl: ctrl, conn: nc, httpAddr: httpAddr}, nil
}

// Start subscribes to lifecycle events and starts the HTTP server.
// It blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	sub, err := s.conn.Subscribe(lifecycleSubject, s.handleEvent)
	if err != nil {
		return err
	}
	log.Printf("orchestrator: subscribed to NATS subject %q", lifecycleSubject)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ready", s.readyHandler)

	srv := &http.Server{
		Addr:         s.httpAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("orchestrator: HTTP API listening on %s", s.httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("orchestrator: HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()

	_ = sub.Drain()
	s.conn.Close()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

// handleEvent processes a lifecycle audit event and creates or deletes flow Deployments.
func (s *Server) handleEvent(msg *nats.Msg) {
	var ev LifecycleEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		log.Printf("orchestrator: parse event: %v", err)
		return
	}
	// Only process lifecycle events (node_type == "lifecycle").
	if ev.NodeType != "lifecycle" {
		return
	}
	action, _ := ev.Input["action"].(string)

	switch action {
	case "deployed", "reloaded":
		if err := s.ctrl.Deploy(context.Background(), ev.FlowID); err != nil {
			log.Printf("orchestrator: deploy flow %q: %v", ev.FlowID, err)
		}
	case "stopped":
		if err := s.ctrl.Stop(context.Background(), ev.FlowID); err != nil {
			log.Printf("orchestrator: stop flow %q: %v", ev.FlowID, err)
		}
	}
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "orchestrator"})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.conn == nil || s.conn.IsClosed() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "service": "orchestrator"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready", "service": "orchestrator"})
}
