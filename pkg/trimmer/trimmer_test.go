package trimmer_test

import (
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

func TestTrimmer_TrimBytes(t *testing.T) {
	trm := trimmer.New()

	input := []byte("func main()   {\n\n\n\n   fmt.Println(\"hello\")   \n}")
	output, saved := trm.TrimBytes(input)

	if saved <= 0 {
		t.Errorf("expected saved bytes > 0, got %d", saved)
	}

	if len(output) >= len(input) {
		t.Errorf("expected output length < input length")
	}
}

func BenchmarkTrimmer_TrimBytes(b *testing.B) {
	trm := trimmer.New()
	input := []byte("func main()   {\n\n\n\n   fmt.Println(\"hello world testing\")   \n}")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = trm.TrimBytes(input)
	}
}
