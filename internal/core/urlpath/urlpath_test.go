package urlpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodePathKeepsSlashesAndUnreservedCharacters(t *testing.T) {
	assert.Equal(t, "src/main.rs", EncodePath("src/main.rs"))
	assert.Equal(t, "a-b_c.d~e/F9", EncodePath("a-b_c.d~e/F9"))
}

func TestEncodePathEscapesReservedCharactersInUppercaseHex(t *testing.T) {
	assert.Equal(t, "a%20b%23", EncodePath("a b#"))
	assert.Equal(t, "%C3%B1", EncodePath("ñ"))
}

func TestSpecialCharactersRoundTrip(t *testing.T) {
	for _, path := range []string{"a b/c#d?.md", "ñandú/é.txt", "100%/x", "a+b/[c].rs"} {
		assert.Equal(t, path, DecodePath(EncodePath(path)), path)
	}
}

func TestDecodePathKeepsInvalidEscapes(t *testing.T) {
	assert.Equal(t, "100%", DecodePath("100%"))
	assert.Equal(t, "%zz", DecodePath("%zz"))
	assert.Equal(t, "%2", DecodePath("%2"))
}

// The original parser used Rust's from_str_radix, which accepts a leading '+' in the
// two-digit escape. The port keeps that behavior so existing URLs decode the same way.
func TestDecodePathAcceptsPlusSignInsideEscapeLikeTheOriginal(t *testing.T) {
	assert.Equal(t, "\x01", DecodePath("%+1"))
	assert.Equal(t, "%++", DecodePath("%++"))
}
