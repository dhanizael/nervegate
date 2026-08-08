package classifier

import (
	"strings"
)

// Tier represents the routed model category.
type Tier string

const (
	TierFast      Tier = "FAST"      // e.g. Gemini Flash, DeepSeek-V3, Haiku
	TierStandard  Tier = "STANDARD"  // e.g. Claude 3.5 Sonnet, GPT-4o
	TierReasoning Tier = "REASONING" // e.g. Claude 3.7 Sonnet Thinking, O3-Mini
)

// ClassificationResult contains the calculated complexity score and routed tier.
type ClassificationResult struct {
	Score       int    `json:"score"`       // 0 - 100
	Tier        Tier   `json:"tier"`        // FAST, STANDARD, REASONING
	Reason      string `json:"reason"`      // Explanation of classification
	Criticality bool   `json:"criticality"` // High urgency/criticality flag
}

// Classifier inspects request payloads and computes complexity metrics.
type Classifier struct{}

// New returns a new instance of Classifier.
func New() *Classifier {
	return &Classifier{}
}

// Classify evaluates the request prompt and tool context.
func (c *Classifier) Classify(prompt string, toolContext string) ClassificationResult {
	score := 0
	var reasons []string
	critical := false

	length := len(prompt) + len(toolContext)
	if length > 8000 {
		score += 35
		reasons = append(reasons, "Large payload context (>8k chars)")
	} else if length > 2000 {
		score += 20
		reasons = append(reasons, "Medium payload context (>2k chars)")
	} else {
		score += 5
	}

	lowerPrompt := strings.ToLower(prompt)

	// Criticality keywords
	criticalKeywords := []string{"[critical]", "[architect]", "memory leak", "race condition", "security vulnerability", "deadlock", "refactor architecture"}
	for _, kw := range criticalKeywords {
		if strings.Contains(lowerPrompt, kw) {
			score += 30
			critical = true
			reasons = append(reasons, "Contains critical keyword: "+kw)
			break
		}
	}

	// Code / AST indicators
	codeKeywords := []string{"func ", "def ", "class ", "type ", "struct ", "interface ", "impl ", "async "}
	codeCount := 0
	for _, kw := range codeKeywords {
		if strings.Contains(lowerPrompt, kw) {
			codeCount++
		}
	}
	if codeCount >= 2 {
		score += 20
		reasons = append(reasons, "High code syntax density")
	}

	// Determine final Tier
	var tier Tier
	switch {
	case score >= 60 || critical:
		tier = TierReasoning
	case score >= 25:
		tier = TierStandard
	default:
		tier = TierFast
	}

	return ClassificationResult{
		Score:       score,
		Tier:        tier,
		Reason:      strings.Join(reasons, "; "),
		Criticality: critical,
	}
}
