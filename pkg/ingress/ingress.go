package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dhanizael/nervegate/pkg/classifier"
	"github.com/dhanizael/nervegate/pkg/rotator"
	"github.com/dhanizael/nervegate/pkg/trimmer"
)

const (
	defaultUpstream   = "https://api.openai.com"
	defaultMaxBody    = 32 << 20 // 32 MiB
	defaultSocketMode = 0o600
	cooldownOn429     = 30 * time.Second
	cooldownOn5xx     = 10 * time.Second
)

// IngressServer manages HTTP and Unix Domain Socket listeners for NerveGate.
type IngressServer struct {
	cfg      Config
	cls      *classifier.Classifier
	rot      *rotator.KeyRotator
	trm      *trimmer.Trimmer
	upstream *url.URL
	httpSrv  *http.Server
}

// Config holds configuration parameters for the IngressServer.
type Config struct {
	Port         int
	SocketPath   string
	SocketMode   os.FileMode   // mode for the unix socket; defaults to 0600
	Provider     string        // provider pool name used with the KeyRotator; defaults to "openai"
	Upstream     string        // upstream base URL; defaults to https://api.openai.com
	MaxBodyBytes int64         // request body cap; defaults to 32 MiB
	Cooldown429  time.Duration // key cool-down on HTTP 429; defaults to 30s
	Cooldown5xx  time.Duration // key cool-down on HTTP 5xx; defaults to 10s
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

// NewServer creates a new IngressServer instance.
func NewServer(cfg Config, cls *classifier.Classifier, rot *rotator.KeyRotator, trm *trimmer.Trimmer) *IngressServer {
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	if cfg.Upstream == "" {
		cfg.Upstream = defaultUpstream
	}
	if cfg.SocketMode == 0 {
		cfg.SocketMode = defaultSocketMode
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBody
	}
	if cfg.Cooldown429 <= 0 {
		cfg.Cooldown429 = cooldownOn429
	}
	if cfg.Cooldown5xx <= 0 {
		cfg.Cooldown5xx = cooldownOn5xx
	}
	u, err := url.Parse(cfg.Upstream)
	if err != nil || u.Scheme == "" || u.Host == "" {
		u, _ = url.Parse(defaultUpstream)
	}
	return &IngressServer{
		cfg:      cfg,
		cls:      cls,
		rot:      rot,
		trm:      trm,
		upstream: u,
	}
}

func (s *IngressServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "NerveGate Gateway Engine",
	})
}

// headerInjector adds extra headers before the first write, guaranteeing they
// reach the client even on error paths (where ModifyResponse is not invoked).
type headerInjector struct {
	http.ResponseWriter
	extra http.Header
	once  sync.Once
}

func (h *headerInjector) inject() {
	h.once.Do(func() {
		for k, vv := range h.extra {
			for _, v := range vv {
				h.Header().Add(k, v)
			}
		}
	})
}

func (h *headerInjector) WriteHeader(code int) {
	h.inject()
	h.ResponseWriter.WriteHeader(code)
}

func (h *headerInjector) Write(b []byte) (int, error) {
	h.inject()
	return h.ResponseWriter.Write(b)
}

// usageBody is the minimal slice of a chat completion response used for TPM
// accounting.
type usageBody struct {
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// recordTokens parses usage.total_tokens from a successful upstream response
// and records it against the key used for the request. The full response body
// is buffered and re-wrapped so the client receives it intact.
func (s *IngressServer) recordTokens(res *http.Response, key *rotator.APIKey) {
	if res.StatusCode != http.StatusOK || key == nil || key.TPM <= 0 {
		return
	}
	ct := res.Header.Get("Content-Type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil || mt != "application/json" {
		return
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}
	res.Body = io.NopCloser(bytes.NewReader(body))

	var u usageBody
	if json.Unmarshal(body, &u) != nil || u.Usage.TotalTokens <= 0 {
		return
	}
	s.rot.MarkTokens(s.cfg.Provider, key.ID, u.Usage.TotalTokens)
}

// handleChatCompletions routes and proxies /v1/chat/completions requests.
func (s *IngressServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	bodyBytes, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body exceeds maximum allowed size", "invalid_request_error")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "failed to read request body", "invalid_request_error")
		return
	}
	if len(bodyBytes) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty request body", "invalid_request_error")
		return
	}

	// 1. Trim payload
	trimmedBody, savedBytes := s.trm.TrimBytes(bodyBytes)

	// 2. Parse prompt context for classification
	var req ChatCompletionRequest
	if err := json.Unmarshal(trimmedBody, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body", "invalid_request_error")
		return
	}

	promptContext := ""
	if len(req.Messages) > 0 {
		promptContext = req.Messages[len(req.Messages)-1].Content
	}

	// 3. Classify task complexity & criticality
	clsRes := s.cls.Classify(promptContext, "")

	// 4. Select an upstream key from the pool. When no key pool is configured
	// for the provider, fall back to the caller's own Authorization header.
	// When a pool IS configured but every key is rate limited or cooling down,
	// fail fast instead of silently leaking the request upstream.
	var usedKey *rotator.APIKey
	key, keyErr := s.rot.GetKey(s.cfg.Provider)
	switch {
	case keyErr == nil:
		usedKey = key
		r.Header.Set("Authorization", "Bearer "+key.Key)
	case s.rot.HasKeys(s.cfg.Provider):
		writeJSONError(w, http.StatusServiceUnavailable, "all API keys for provider "+s.cfg.Provider+" are rate limited or cooling down", "rate_limit_error")
		return
	default:
		// No pool configured: pass through the caller's own credentials.
	}

	proxy := httputil.NewSingleHostReverseProxy(s.upstream)
	r.Body = io.NopCloser(bytes.NewReader(trimmedBody))
	r.ContentLength = int64(len(trimmedBody))
	r.Host = s.upstream.Host

	proxy.ModifyResponse = func(res *http.Response) error {
		// Inject NerveGate metadata response headers (ASCII names only).
		elapsed := time.Since(start)
		res.Header.Set("X-NerveGate-Latency-Us", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1000.0))
		res.Header.Set("X-NerveGate-Tier", string(clsRes.Tier))
		res.Header.Set("X-NerveGate-Score", strconv.Itoa(clsRes.Score))
		res.Header.Set("X-NerveGate-Trimmed-Bytes", strconv.Itoa(savedBytes))

		if usedKey != nil {
			switch {
			case res.StatusCode == http.StatusTooManyRequests:
				s.rot.MarkCoolDown(s.cfg.Provider, usedKey.ID, s.cfg.Cooldown429)
			case res.StatusCode >= 500:
				s.rot.MarkCoolDown(s.cfg.Provider, usedKey.ID, s.cfg.Cooldown5xx)
			}
			s.recordTokens(res, usedKey)
		}
		return nil
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		elapsed := time.Since(start)
		meta := http.Header{}
		meta.Set("X-NerveGate-Latency-Us", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1000.0))
		meta.Set("X-NerveGate-Tier", string(clsRes.Tier))
		meta.Set("X-NerveGate-Score", strconv.Itoa(clsRes.Score))
		meta.Set("X-NerveGate-Trimmed-Bytes", strconv.Itoa(savedBytes))
		rw = &headerInjector{ResponseWriter: rw, extra: meta}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(rw, `{"error":{"message":"upstream request failed","type":"upstream_error"}}`)
	}

	proxy.ServeHTTP(w, r)
}

func writeJSONError(w http.ResponseWriter, status int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    errType,
		},
	})
}

// newMux wires the HTTP routes. Kept separate from Start so routing is testable.
func (s *IngressServer) newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/route", s.handleChatCompletions)
	return mux
}

// Start launches TCP HTTP and Unix Domain Socket servers.
func (s *IngressServer) Start(ctx context.Context) error {
	s.httpSrv = &http.Server{
		Handler:      s.newMux(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 2)
	var unixListener net.Listener

	// 1. Start Unix Domain Socket listener if specified
	if s.cfg.SocketPath != "" {
		if err := os.Remove(s.cfg.SocketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale socket %s: %w", s.cfg.SocketPath, err)
		}
		var err error
		unixListener, err = net.Listen("unix", s.cfg.SocketPath)
		if err != nil {
			return fmt.Errorf("failed to listen on unix socket %s: %w", s.cfg.SocketPath, err)
		}
		if err := os.Chmod(s.cfg.SocketPath, s.cfg.SocketMode); err != nil {
			_ = unixListener.Close()
			return fmt.Errorf("failed to set socket mode on %s: %w", s.cfg.SocketPath, err)
		}
		defer os.Remove(s.cfg.SocketPath)
	}

	// 2. Start TCP HTTP listener
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		if unixListener != nil {
			_ = unixListener.Close()
		}
		return fmt.Errorf("failed to listen on TCP addr %s: %w", addr, err)
	}

	if unixListener != nil {
		go func() {
			errCh <- s.httpSrv.Serve(unixListener)
		}()
	}
	go func() {
		errCh <- s.httpSrv.Serve(tcpListener)
	}()

	// 3. Run until context cancellation or a fatal serve error
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}
