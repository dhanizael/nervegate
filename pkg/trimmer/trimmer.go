package trimmer

import (
	"regexp"
	"strings"
)

var (
	multiSpaceRegex = regexp.MustCompile(`[ \t]{2,}`)
	emptyLinesRegex = regexp.MustCompile(`\n{3,}`)
)

// Trimmer compresses tool output payloads to save token budget.
type Trimmer struct{}

// New returns a new Trimmer instance.
func New() *Trimmer {
	return &Trimmer{}
}

// Trim compresses whitespace, duplicate empty lines, and redundant log headers.
func (t *Trimmer) Trim(content string) (string, int) {
	originalLen := len(content)
	if originalLen == 0 {
		return content, 0
	}

	// 1. Collapse horizontal spaces
	result := multiSpaceRegex.ReplaceAllString(content, " ")

	// 2. Collapse excess newlines (> 2 newlines -> 2 newlines)
	result = emptyLinesRegex.ReplaceAllString(result, "\n\n")

	// 3. Trim leading/trailing whitespace per line
	lines := strings.Split(result, "\n")
	trimmedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmedLines = append(trimmedLines, strings.TrimRight(line, " \t"))
	}
	result = strings.Join(trimmedLines, "\n")

	savedBytes := originalLen - len(result)
	if savedBytes < 0 {
		savedBytes = 0
	}
	return result, savedBytes
}
