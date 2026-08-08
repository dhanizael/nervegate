package classifier_test

import (
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/classifier"
)

func TestClassifier_Classify(t *testing.T) {
	cls := classifier.New()

	t.Run("Fast Tier Simple Prompt", func(t *testing.T) {
		res := cls.Classify("hello world", "")
		if res.Tier != classifier.TierFast {
			t.Errorf("expected TierFast, got %s", res.Tier)
		}
	})

	t.Run("Reasoning Tier Critical Keyword", func(t *testing.T) {
		res := cls.Classify("Fix [CRITICAL] deadlock in mutex locking", "")
		if res.Tier != classifier.TierReasoning {
			t.Errorf("expected TierReasoning, got %s", res.Tier)
		}
		if !res.Criticality {
			t.Errorf("expected Criticality = true")
		}
	})
}
