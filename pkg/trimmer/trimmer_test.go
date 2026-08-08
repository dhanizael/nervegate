package trimmer_test

import (
	"testing"

	"github.com/hxmdxnx/nervegate/pkg/trimmer"
)

func TestTrimmer_Trim(t *testing.T) {
	trm := trimmer.New()

	input := "func main()  {\n\n\n\tfmt.Println(\"hello\")   \n}"
	output, savedBytes := trm.Trim(input)

	if savedBytes <= 0 {
		t.Errorf("expected savedBytes > 0, got %d", savedBytes)
	}

	if len(output) >= len(input) {
		t.Errorf("expected trimmed output length to be smaller than input length")
	}
}
