package rotator

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// rateLimitWindow is the sliding-window duration used for RPM/TPM accounting.
const rateLimitWindow = time.Minute

// APIKey represents an individual key in a provider's key pool.
type APIKey struct {
	ID       string `json:"id"`
	Key      string `json:"-"`
	Provider string `json:"provider"`
	RPM      int    `json:"rpm"` // max requests per minute; 0 = unlimited
	TPM      int    `json:"tpm"` // max tokens per minute; 0 = unlimited

	CoolUntil time.Time `json:"cool_until"`

	// requestTimes and tokenUsage implement the sliding window. They are only
	// mutated while holding the owning KeyRotator's lock.
	requestTimes []time.Time
	tokenUsage   []tokenUsage
}

type tokenUsage struct {
	at     time.Time
	tokens int
}

// KeyRotator manages round-robin rotation and rate-limit cool-downs across API keys.
type KeyRotator struct {
	mu   sync.RWMutex
	keys map[string][]*APIKey // provider -> keys
	idx  map[string]int       // provider -> current index
}

// New returns a new KeyRotator instance.
func New() *KeyRotator {
	return &KeyRotator{
		keys: make(map[string][]*APIKey),
		idx:  make(map[string]int),
	}
}

// AddKey adds an API key to the specified provider pool.
func (r *KeyRotator) AddKey(provider string, key *APIKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key.ID == "" {
		key.ID = fmt.Sprintf("%s-%d", provider, len(r.keys[provider])+1)
	}
	r.keys[provider] = append(r.keys[provider], key)
}

// HasKeys reports whether any keys are registered for the provider.
func (r *KeyRotator) HasKeys(provider string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.keys[provider]) > 0
}

// pruneWindow drops sliding-window entries older than the window, copying to a
// fresh slice when entries were pruned so expired records and their backing
// arrays can be garbage collected. Callers must hold the lock.
func pruneWindow(now time.Time, times []time.Time, tokens []tokenUsage) ([]time.Time, []tokenUsage) {
	cutoff := now.Add(-rateLimitWindow)
	i := 0
	for i < len(times) && times[i].Before(cutoff) {
		i++
	}
	j := 0
	for j < len(tokens) && tokens[j].at.Before(cutoff) {
		j++
	}
	if i > 0 {
		times = append([]time.Time(nil), times[i:]...)
	}
	if j > 0 {
		tokens = append([]tokenUsage(nil), tokens[j:]...)
	}
	return times, tokens
}

// healthy reports whether a key is currently usable (not cooling down and
// within its RPM/TPM sliding-window limits). Callers must hold the lock.
func healthy(now time.Time, key *APIKey) bool {
	if now.Before(key.CoolUntil) {
		return false
	}
	if key.RPM > 0 && len(key.requestTimes) >= key.RPM {
		return false
	}
	if key.TPM > 0 {
		var used int
		for _, tu := range key.tokenUsage {
			used += tu.tokens
		}
		if used >= key.TPM {
			return false
		}
	}
	return true
}

// GetKey retrieves the next available, healthy API key for a provider,
// round-robin across the pool. It records the request for RPM accounting.
func (r *KeyRotator) GetKey(provider string) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pool, exists := r.keys[provider]
	if !exists || len(pool) == 0 {
		return nil, errors.New("no API keys available for provider: " + provider)
	}

	now := time.Now()
	startIdx := r.idx[provider]
	total := len(pool)

	for i := 0; i < total; i++ {
		curr := (startIdx + i) % total
		key := pool[curr]
		if key.RPM > 0 {
			key.requestTimes, _ = pruneWindow(now, key.requestTimes, nil)
		}
		if key.TPM > 0 {
			_, key.tokenUsage = pruneWindow(now, nil, key.tokenUsage)
		}

		if healthy(now, key) {
			if key.RPM > 0 {
				key.requestTimes = append(key.requestTimes, now)
			}
			r.idx[provider] = (curr + 1) % total
			return key, nil
		}
	}

	return nil, errors.New("all API keys for provider " + provider + " are currently rate limited or cooling down")
}

// MarkTokens records token usage for a key's TPM sliding window (0 tokens is a no-op).
func (r *KeyRotator) MarkTokens(provider string, keyID string, tokens int) {
	if tokens <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	pool := r.keys[provider]
	now := time.Now()
	for _, k := range pool {
		if k.ID == keyID {
			_, k.tokenUsage = pruneWindow(now, nil, k.tokenUsage)
			k.tokenUsage = append(k.tokenUsage, tokenUsage{at: now, tokens: tokens})
			return
		}
	}
}

// MarkCoolDown sets a cool-down timer for a specific key upon hitting rate limits (429)
// or upstream failures (5xx).
func (r *KeyRotator) MarkCoolDown(provider string, keyID string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pool := r.keys[provider]
	for _, k := range pool {
		if k.ID == keyID {
			k.CoolUntil = time.Now().Add(duration)
			return
		}
	}
}
