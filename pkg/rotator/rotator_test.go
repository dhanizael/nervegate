package rotator_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dhanizael/nervegate/pkg/rotator"
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

func TestKeyRotator_RoundRobin(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-1", Provider: "openai"})
	rot.AddKey("openai", &rotator.APIKey{ID: "k2", Key: "sk-2", Provider: "openai"})

	seen := map[string]int{}
	for i := 0; i < 4; i++ {
		key, err := rot.GetKey("openai")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[key.ID]++
	}
	if seen["k1"] != 2 || seen["k2"] != 2 {
		t.Errorf("expected even round-robin k1=2 k2=2, got %v", seen)
	}
}

func TestKeyRotator_RPMSlidingWindow(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-1", Provider: "openai", RPM: 2})

	for i := 0; i < 2; i++ {
		if _, err := rot.GetKey("openai"); err != nil {
			t.Fatalf("request %d should succeed: %v", i+1, err)
		}
	}
	if _, err := rot.GetKey("openai"); err == nil {
		t.Errorf("expected rate-limit error on 3rd request within window")
	}
}

func TestKeyRotator_CoolDown(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-1", Provider: "openai"})

	if _, err := rot.GetKey("openai"); err != nil {
		t.Fatalf("key should be usable before cooldown: %v", err)
	}

	rot.MarkCoolDown("openai", "k1", time.Minute)
	if _, err := rot.GetKey("openai"); err == nil {
		t.Errorf("expected error while key is cooling down")
	}

	rot.MarkCoolDown("openai", "k1", 0)
	if _, err := rot.GetKey("openai"); err != nil {
		t.Errorf("key should be usable after cooldown expires: %v", err)
	}
}

func TestKeyRotator_TPMSlidingWindow(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{ID: "k1", Key: "sk-1", Provider: "openai", TPM: 10})

	rot.MarkTokens("openai", "k1", 6)
	if _, err := rot.GetKey("openai"); err != nil {
		t.Errorf("key should stay healthy under TPM limit: %v", err)
	}

	rot.MarkTokens("openai", "k1", 4)
	if _, err := rot.GetKey("openai"); err == nil {
		t.Errorf("expected rate-limit error once tokens hit the TPM cap")
	}
}

func TestKeyRotator_AutoID(t *testing.T) {
	rot := rotator.New()
	rot.AddKey("openai", &rotator.APIKey{Key: "sk-secret-material", Provider: "openai"})

	key, err := rot.GetKey("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID == "" {
		t.Errorf("expected auto-generated key ID")
	}
	if len(key.ID) >= len("sk-secret-material") || key.ID == key.Key {
		t.Errorf("auto ID must not embed key material, got %q", key.ID)
	}
}
