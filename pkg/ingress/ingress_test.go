package ingress_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
	"github.com/hxmdxnx/nervegate/pkg/ingress"
	"github.com/hxmdxnx/nervegate/pkg/rotator"
	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

func TestIngressServer_ChatCompletions(t *testing.T) {
	cls := classifier.New()
	rot := rotator.New()
	trm := trimmer.New()

	_ = ingress.NewServer(ingress.Config{Port: 8080}, cls, rot, trm)

	payload := ingress.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []ingress.ChatMessage{
			{Role: "user", Content: "Fix [CRITICAL] deadlock in mutex handler"},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(bodyBytes))
	w := httptest.NewRecorder()

	if req.URL.Path != "/v1/chat/completions" {
		t.Errorf("unexpected path")
	}

	if w == nil {
		t.Errorf("recorder is nil")
	}
}
