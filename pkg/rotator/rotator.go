package rotator

import (
	"errors"
	"sync"
	"time"
)

// APIKey represents an individual key in a provider's key pool.
type APIKey struct {
	ID        string    `json:"id"`
	Key       string    `json:"-"`
	Provider  string    `json:"provider"`
	RPM       int       `json:"rpm"`
	CoolUntil time.Time `json:"cool_until"`
}

// KeyRotator manages round-robin rotation and rate-limit cool-downs across API keys.
type KeyRotator struct {
	mu   sync.RWMutex
	keys map[string][]*APIKey // provider -> keys
	idx  map[string]int      // provider -> current index
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
	r.keys[provider] = append(r.keys[provider], key)
}

// GetKey retrieves the next available, healthy API key for a provider.
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

		if now.After(key.CoolUntil) {
			r.idx[provider] = (curr + 1) % total
			return key, nil
		}
	}

	return nil, errors.New("all API keys for provider " + provider + " are currently rate limited")
}

// MarkCoolDown sets a cool-down timer for a specific key upon hitting rate limits (429).
func (r *KeyRotator) MarkCoolDown(provider string, keyID string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pool := r.keys[provider]
	for _, k := range pool {
		if k.ID == keyID {
			k.CoolUntil = time.Now().Add(duration)
			break
		}
	}
}
