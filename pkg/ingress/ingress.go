package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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

// ChatCompletionRequest defines OpenAI-compatible chat completion payload.
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (s *IngressServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "NerveGate Gateway Engine",
	})
}

// handleChatCompletions routes and proxies /v1/chat/completions requests.
func (s *IngressServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 {
		http.Error(w, `{"error":{"message":"empty or invalid request body","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}

	// 1. Trim payload
	trimmedBody, savedBytes := s.trm.TrimBytes(bodyBytes)

	// 2. Parse prompt context for classification
	var req ChatCompletionRequest
	_ = json.Unmarshal(trimmedBody, &req)

	promptContext := ""
	if len(req.Messages) > 0 {
		promptContext = req.Messages[len(req.Messages)-1].Content
	}

	// 3. Classify task complexity & criticality
	clsRes := s.cls.Classify(promptContext, "")

	elapsed := time.Since(start)

	// Inject NerveGate metadata response headers
	w.Header().Set("X-NerveGate-Latency-µs", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1000.0))
	w.Header().Set("X-NerveGate-Tier", string(clsRes.Tier))
	w.Header().Set("X-NerveGate-Score", fmt.Sprintf("%d", clsRes.Score))
	w.Header().Set("X-NerveGate-Trimmed-Bytes", fmt.Sprintf("%d", savedBytes))

	// Upstream proxy target (OpenAI compatible endpoint or configured upstream provider)
	upstreamURL, err := url.Parse("https://api.openai.com")
	if err != nil {
		http.Error(w, `{"error":{"message":"invalid upstream config"}}`, http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	r.Body = io.NopCloser(bytes.NewReader(trimmedBody))
	r.ContentLength = int64(len(trimmedBody))
	r.Host = upstreamURL.Host

	proxy.ServeHTTP(w, r)
}

// Start launches TCP HTTP and Unix Domain Socket servers.
func (s *IngressServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/route", s.handleChatCompletions)

	s.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 1. Start Unix Domain Socket listener if specified
	if s.socketPath != "" {
		_ = os.Remove(s.socketPath)
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
