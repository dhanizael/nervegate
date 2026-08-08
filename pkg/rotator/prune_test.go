package rotator

import (
	"testing"
	"time"
)

func TestPruneWindow_RemovesExpiredEntries(t *testing.T) {
	now := time.Now()
	times := []time.Time{now.Add(-2 * rateLimitWindow), now.Add(-90 * time.Second), now.Add(-30 * time.Second), now}
	tokens := []tokenUsage{
		{at: now.Add(-2 * rateLimitWindow), tokens: 100},
		{at: now.Add(-45 * time.Second), tokens: 50},
	}

	prunedTimes, prunedTokens := pruneWindow(now, times, tokens)

	if len(prunedTimes) != 2 {
		t.Fatalf("expected 2 surviving timestamps, got %d: %v", len(prunedTimes), prunedTimes)
	}
	if !prunedTimes[0].Equal(times[2]) || !prunedTimes[1].Equal(times[3]) {
		t.Errorf("wrong surviving timestamps: %v", prunedTimes)
	}

	if len(prunedTokens) != 1 || prunedTokens[0].tokens != 50 {
		t.Errorf("expected only the recent token usage to survive, got %v", prunedTokens)
	}
}

func TestPruneWindow_NoExpiredEntries(t *testing.T) {
	now := time.Now()
	times := []time.Time{now.Add(-10 * time.Second), now}
	tokens := []tokenUsage{{at: now.Add(-5 * time.Second), tokens: 1}}

	prunedTimes, prunedTokens := pruneWindow(now, times, tokens)

	// No pruning should mean the original slices are returned unchanged.
	if len(prunedTimes) != 2 || len(prunedTokens) != 1 {
		t.Fatalf("unexpected prune result: times=%v tokens=%v", prunedTimes, prunedTokens)
	}
}

func TestPruneWindow_AllExpired(t *testing.T) {
	now := time.Now()
	times := []time.Time{now.Add(-2 * rateLimitWindow), now.Add(-3 * rateLimitWindow)}
	tokens := []tokenUsage{{at: now.Add(-2 * rateLimitWindow), tokens: 7}}

	prunedTimes, prunedTokens := pruneWindow(now, times, tokens)

	if len(prunedTimes) != 0 || len(prunedTokens) != 0 {
		t.Fatalf("expected everything to be pruned, got times=%v tokens=%v", prunedTimes, prunedTokens)
	}
}

func TestHealthy_CooldownAndLimits(t *testing.T) {
	now := time.Now()

	key := &APIKey{CoolUntil: now.Add(time.Minute)}
	if healthy(now, key) {
		t.Errorf("key in cooldown must not be healthy")
	}

	key = &APIKey{RPM: 2, requestTimes: []time.Time{now, now.Add(-time.Second)}}
	if healthy(now, key) {
		t.Errorf("key at RPM cap must not be healthy")
	}

	key = &APIKey{TPM: 10, tokenUsage: []tokenUsage{{at: now, tokens: 10}}}
	if healthy(now, key) {
		t.Errorf("key at TPM cap must not be healthy")
	}

	key = &APIKey{RPM: 2, TPM: 10, requestTimes: []time.Time{now}, tokenUsage: []tokenUsage{{at: now, tokens: 4}}}
	if !healthy(now, key) {
		t.Errorf("key under all limits should be healthy")
	}
}
