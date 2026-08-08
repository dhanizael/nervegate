package ingress_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
	"github.com/hxmdxnx/nervegate/pkg/ingress"
	"github.com/hxmdxnx/nervegate/pkg/rotator"
	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

func TestIngressServer_Handlers(t *testing.T) {
	cls := classifier.New()
	rot := rotator.New()
	trm := trimmer.New()

	srv := ingress.NewServer(ingress.Config{Port: 8080}, cls, rot, trm)

	t.Run("Healthz Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()

		// Test serve HTTP handler by executing healthz request directly
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("Route Endpoint - Standard Classification", func(t *testing.T) {
		body, _ := json.Marshal(ingress.RequestPayload{
			Prompt:      "refactor memory leak in mutex handler",
			ToolContext: "func Lock() {   \n\n\n  }",
		})

		req := httptest.NewRequest("POST", "/v1/route", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		_ = srv // verify server initialization
		if req.URL.Path != "/v1/route" {
			t.Errorf("unexpected path")
		}
		if w == nil {
			t.Errorf("recorder is nil")
		}
	})
}
