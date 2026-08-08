package trimmer

import (
	"bytes"
	"sync"
)

// maxPoolBufferSize caps the retained capacity of pooled buffers. Buffers that
// grew beyond this (e.g. from one huge payload) are discarded instead of being
// returned to the pool, so a single oversized request cannot permanently pin
// memory in the process.
const maxPoolBufferSize = 64 << 10 // 64 KiB

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Trimmer performs allocation-light byte slice transformations on tool outputs.
type Trimmer struct{}

// New returns a new Trimmer instance.
func New() *Trimmer {
	return &Trimmer{}
}

// TrimBytes collapses runs of whitespace and duplicate newlines from byte
// payloads. It reuses pooled buffers internally; the returned slice is a fresh
// copy so callers may retain it safely.
func (t *Trimmer) TrimBytes(input []byte) ([]byte, int) {
	origLen := len(input)
	if origLen == 0 {
		return input, 0
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Grow(origLen)
	defer func() {
		// Only keep buffers within the size cap in the pool.
		if buf.Cap() <= maxPoolBufferSize {
			bufferPool.Put(buf)
		}
	}()

	lastWasSpace := false
	newlineCount := 0

	for i := 0; i < len(input); i++ {
		b := input[i]

		if b == '\n' {
			newlineCount++
			if newlineCount <= 2 {
				buf.WriteByte(b)
			}
			lastWasSpace = false
			continue
		}

		newlineCount = 0

		if b == ' ' || b == '\t' {
			if !lastWasSpace {
				buf.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}

		lastWasSpace = false
		buf.WriteByte(b)
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	saved := origLen - len(result)
	if saved < 0 {
		saved = 0
	}
	return result, saved
}

// Trim wraps TrimBytes for string inputs.
func (t *Trimmer) Trim(content string) (string, int) {
	if len(content) == 0 {
		return content, 0
	}
	trimmedBytes, saved := t.TrimBytes([]byte(content))
	return string(trimmedBytes), saved
}
