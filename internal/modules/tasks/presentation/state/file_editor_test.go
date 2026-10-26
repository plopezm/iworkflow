package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// A browser sends a textarea with LF line endings, so the editor compares and writes text without CRLF, and remembers
// the ending of the file it loaded.
func TestContentDigestIgnoresLineEndings(t *testing.T) {
	t.Parallel()
	assert.Equal(t, state.ContentDigest("a\nb\n"), state.ContentDigest("a\r\nb\r\n"))
	assert.False(t, state.IsDirty("a\nb\n", state.ContentDigest("a\r\nb\r\n")))
	assert.True(t, state.IsDirty("a\nc\n", state.ContentDigest("a\r\nb\r\n")))
}

func TestLineEndingOfReadsTheEndingOfTheFile(t *testing.T) {
	t.Parallel()
	assert.Equal(t, state.LineEndingCRLF, state.LineEndingOf("a\r\nb\r\n"))
	assert.Equal(t, state.LineEndingLF, state.LineEndingOf("a\nb\n"))
	assert.Equal(t, state.LineEndingLF, state.LineEndingOf(""))
}

// A file with CRLF gets CRLF on every line, whether or not the text already holds it; an LF file gets LF.
func TestApplyLineEndingWritesTheEndingOfTheFile(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a\r\nb\r\n", state.ApplyLineEnding("a\nb\n", state.LineEndingCRLF))
	assert.Equal(t, "a\r\nb\r\n", state.ApplyLineEnding("a\r\nb\r\n", state.LineEndingCRLF))
	assert.Equal(t, "a\nb\n", state.ApplyLineEnding("a\r\nb\n", state.LineEndingLF))
}
