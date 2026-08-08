package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
	"github.com/hxmdxnx/nervegate/pkg/rotator"
	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

// IngressServer manages HTTP and Unix Domain Socket listeners for NerveGate.
type IngressServer struct {
	port       int
	socketPath string
	cls        *classifier.Classifier
	rot        *rotator.KeyRotator
	trm        *trimmer.Trimmer
	httpSrv    *http.Server
}

// Config holds configuration parameters for the IngressServer.
type Config struct {
	Port       int
	SocketPath string
}

// NewServer creates a new IngressServer instance.
func NewServer(cfg Config, cls *classifier.Classifier, rot *rotator.KeyRotator, trm *trimmer.Trimmer) *IngressServer {
	return &IngressServer{
		port:       cfg.Port,
		socketPath: cfg.SocketPath,
		cls:        cls,
		rot:        rot,
		trm:        trm,
	}
}

// RequestPayload represents incoming proxy requests.
type RequestPayload struct {
	Prompt      string `json:"prompt"`
	ToolContext string `json:"tool_context,omitempty"`
	Provider    string `json:"provider,omitempty"`
}

// ResponsePayload represents the classified & routed response header/metadata.
type ResponsePayload struct {
	Status        string                           `json:"status"`
	Classification classifier.ClassificationResult `json:"classification"`
	TrimmedBytes  int                              `json:"trimmed_bytes"`
	Message       string                           `json:"message"`
}

func (s *IngressServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "NerveGate Sub-Millisecond Gateway",
	})
}

func (s *IngressServer) handleRoute(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON request body"}`, http.StatusBadRequest)
		return
	}

	// 1. Trim payload
	trimmedContext, savedBytes := s.trm.Trim(req.ToolContext)

	// 2. Classify task complexity & criticality
	clsRes := s.cls.Classify(req.Prompt, trimmedContext)

	elapsed := time.Since(start)

	resp := ResponsePayload{
		Status:        "routed",
		Classification: clsRes,
		TrimmedBytes:  savedBytes,
		Message:       fmt.Sprintf("NerveGate routed request in %v (Tier: %s, Score: %d)", elapsed, clsRes.Tier, clsRes.Score),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-NerveGate-Latency", elapsed.String())
	w.Header().Set("X-NerveGate-Tier", string(clsRes.Tier))
	_ = json.NewEncoder(w).Encode(resp)
}

// Start launches TCP HTTP and Unix Domain Socket servers.
func (s *IngressServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/route", s.handleRoute)

	s.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 1. Start Unix Domain Socket listener if specified
	if s.socketPath != "" {
		_ = os.Remove(s.socketPath) // Clean up old socket file if present
		unixListener, err := net.Listen("unix", s.socketPath)
		if err != nil {
			return fmt.Errorf("failed to listen on unix socket %s: %w", s.socketPath, err)
		}
		_ = os.Chmod(s.socketPath, 0666)

		go func() {
			_ = s.httpSrv.Serve(unixListener)
		}()
	}

	// 2. Start TCP HTTP listener
	addr := fmt.Sprintf(":%d", s.port)
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on TCP addr %s: %w", addr, err)
	}

	go func() {
		_ = s.httpSrv.Serve(tcpListener)
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}
