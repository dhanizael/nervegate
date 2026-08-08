package rotator_test

import (
	"sync"
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/rotator"
)

func TestKeyRotator_Concurrent(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-1", Provider: "openai"})
	rot.AddKey("openai", &rotator.APIKey{ID: "k2", Key: "sk-2", Provider: "openai"})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, err := rot.GetKey("openai")
			if err != nil || key == nil {
				t.Errorf("unexpected error fetching key: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestKeyRotator_EmptyPool(t *testing.T) {
	rot := rotator.New()
	_, err := rot.GetKey("anthropic")
	if err == nil {
		t.Errorf("expected error for empty pool, got nil")
	}
}
