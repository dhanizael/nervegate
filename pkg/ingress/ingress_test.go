package ingress

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/dhanizael/nervegate/pkg/classifier"
	"github.com/dhanizael/nervegate/pkg/rotator"
	"github.com/dhanizael/nervegate/pkg/trimmer"
)

func newTestServer(t *testing.T, upstream http.HandlerFunc, cfgOverrides Config) (*IngressServer, *rotator.KeyRotator, *httptest.Server) {
	t.Helper()
	cfg := Config{}
	var up *httptest.Server
	if upstream != nil {
		up = httptest.NewServer(upstream)
		t.Cleanup(up.Close)
		cfg.Upstream = up.URL
	}
	if cfgOverrides.MaxBodyBytes != 0 {
		cfg.MaxBodyBytes = cfgOverrides.MaxBodyBytes
	}
	if cfgOverrides.Provider != "" {
		cfg.Provider = cfgOverrides.Provider
	}

	cls := classifier.New()
	rot := rotator.New()
	trm := trimmer.New()
	srv := NewServer(cfg, cls, rot, trm)
	return srv, rot, up
}

func chatBody(t *testing.T, content string) []byte {
	t.Helper()
	b, err := json.Marshal(ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []ChatMessage{{Role: "user", Content: content}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func doRequest(t *testing.T, srv *IngressServer, body []byte, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	srv.handleChatCompletions(rec, req)
	return rec
}

func TestChatCompletions_KeyRotation(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[],"usage":{"total_tokens":5}}`))
	})

	srv, rot, _ := newTestServer(t, up, Config{})
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-aaa", Provider: "openai"})
	rot.AddKey("openai", &rotator.APIKey{ID: "k2", Key: "sk-bbb", Provider: "openai"})

	body := chatBody(t, "hello")
	if rec := doRequest(t, srv, body, "Bearer client-key"); rec.Code != http.StatusOK {
		t.Fatalf("request 1 status = %d", rec.Code)
	}
	if rec := doRequest(t, srv, body, "Bearer client-key"); rec.Code != http.StatusOK {
		t.Fatalf("request 2 status = %d", rec.Code)
	}

	want := []string{"Bearer sk-aaa", "Bearer sk-bbb"}
	for i := range want {
		if len(seen) <= i || seen[i] != want[i] {
			t.Errorf("request %d auth = %q, want %q (seen=%v)", i+1, safeAt(seen, i), want[i], seen)
		}
	}
}

func TestChatCompletions_PassThroughWithoutPool(t *testing.T) {
	var mu sync.Mutex
	var seenAuth string
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[]}`))
	})

	srv, _, _ := newTestServer(t, up, Config{})
	rec := doRequest(t, srv, chatBody(t, "hello"), "Bearer caller-own-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if seenAuth != "Bearer caller-own-key" {
		t.Errorf("expected caller key pass-through, got %q", seenAuth)
	}
}

func TestChatCompletions_ExhaustedPoolReturns503(t *testing.T) {
	calls := 0
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			t.Error("upstream must not be reached when the key pool is exhausted")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[]}`))
	})

	srv, rot, _ := newTestServer(t, up, Config{})
	// One key with RPM 1; the first request consumes it.
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-aaa", Provider: "openai", RPM: 1})

	if rec := doRequest(t, srv, chatBody(t, "hello"), "Bearer caller-own-key"); rec.Code != http.StatusOK {
		t.Fatalf("request 1 status = %d, want 200", rec.Code)
	}

	// Second request: pool configured but exhausted -> 503, no pass-through.
	rec := doRequest(t, srv, chatBody(t, "hello"), "Bearer caller-own-key")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestChatCompletions_429MarksCooldown(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)
	})

	srv, rot, _ := newTestServer(t, up, Config{})
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-aaa", Provider: "openai"})
	rot.AddKey("openai", &rotator.APIKey{ID: "k2", Key: "sk-bbb", Provider: "openai"})

	if rec := doRequest(t, srv, chatBody(t, "hello"), ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}

	// k1 must now be cooling down, so the next key served is k2.
	key, err := rot.GetKey("openai")
	if err != nil {
		t.Fatalf("rotator should still have a healthy key: %v", err)
	}
	if key.ID != "k2" {
		t.Errorf("expected k2 after k1 cooldown, got %s", key.ID)
	}
}

func TestChatCompletions_5xxMarksCooldown(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	srv, rot, _ := newTestServer(t, up, Config{})
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-aaa", Provider: "openai"})
	rot.AddKey("openai", &rotator.APIKey{ID: "k2", Key: "sk-bbb", Provider: "openai"})

	if rec := doRequest(t, srv, chatBody(t, "hello"), ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	key, err := rot.GetKey("openai")
	if err != nil {
		t.Fatalf("rotator should still have a healthy key: %v", err)
	}
	if key.ID != "k2" {
		t.Errorf("expected k2 after k1 5xx cooldown, got %s", key.ID)
	}
}

func TestChatCompletions_UpstreamErrorPath(t *testing.T) {
	// Reserve a port and close it so the proxy gets a connection-refused error.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := l.Addr().String()
	_ = l.Close()

	srv, _, _ := newTestServer(t, nil, Config{})
	// Point the server at the dead address directly.
	srv.upstream, _ = url.Parse("http://" + deadAddr)

	rec := doRequest(t, srv, chatBody(t, "hello"), "")
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	for _, h := range []string{"X-NerveGate-Latency-Us", "X-NerveGate-Tier", "X-NerveGate-Score"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("expected metadata header %q on error path", h)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestRouting(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[]}`))
	})
	srv, _, _ := newTestServer(t, up, Config{})
	mux := srv.newMux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/route", bytes.NewReader(chatBody(t, "hello"))))
	if rec.Code != http.StatusOK {
		t.Errorf("/v1/route status = %d, want 200", rec.Code)
	}
}

func TestChatCompletions_MetadataHeaders(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[]}`))
	})

	srv, _, _ := newTestServer(t, up, Config{})
	rec := doRequest(t, srv, chatBody(t, "Fix [CRITICAL] deadlock in mutex handler"), "")

	for _, h := range []string{"X-NerveGate-Latency-Us", "X-NerveGate-Tier", "X-NerveGate-Score", "X-NerveGate-Trimmed-Bytes"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("expected metadata header %q to be present", h)
		}
	}
	if tier := rec.Header().Get("X-NerveGate-Tier"); tier != string(classifier.TierReasoning) {
		t.Errorf("tier = %q, want REASONING for critical prompt", tier)
	}
}

func TestChatCompletions_InvalidJSON(t *testing.T) {
	srv, _, _ := newTestServer(t, nil, Config{})
	rec := doRequest(t, srv, []byte(`{not json`), "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestChatCompletions_BodyTooLarge(t *testing.T) {
	srv, _, _ := newTestServer(t, nil, Config{MaxBodyBytes: 16})
	big := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"` + strings.Repeat("x", 64) + `"}]}`)
	rec := doRequest(t, srv, big, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestChatCompletions_TokenAccounting(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","choices":[],"usage":{"total_tokens":6}}`))
	})

	srv, rot, _ := newTestServer(t, up, Config{})
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-aaa", Provider: "openai", TPM: 10})

	if rec := doRequest(t, srv, chatBody(t, "hello"), ""); rec.Code != http.StatusOK {
		t.Fatalf("request 1 status = %d", rec.Code)
	}
	if _, err := rot.GetKey("openai"); err != nil {
		t.Errorf("key should stay healthy after 6/10 tokens: %v", err)
	}

	if rec := doRequest(t, srv, chatBody(t, "hello"), ""); rec.Code != http.StatusOK {
		t.Fatalf("request 2 status = %d", rec.Code)
	}
	if _, err := rot.GetKey("openai"); err == nil {
		t.Errorf("expected rate-limit error after 12/10 tokens")
	}
}

func safeAt(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}
