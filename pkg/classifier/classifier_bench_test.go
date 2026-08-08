package classifier_test

import (
	"testing"

	"github.com/dhanizael/nervegate/pkg/classifier"
)

func BenchmarkClassifier_Classify(b *testing.B) {
	cls := classifier.New()
	prompt := "Fix [CRITICAL] deadlock in mutex handler"
	context := "func Lock() {\n\tmu.Lock()\n}"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cls.Classify(prompt, context)
	}
}
