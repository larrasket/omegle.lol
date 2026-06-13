package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTags_LowercaseTrimDedupe(t *testing.T) {
	out, err := NormalizeTags([]string{"Tech", "  music ", "TECH", "books"}, 10, 30)
	require.NoError(t, err)
	assert.Equal(t, []string{"tech", "music", "books"}, out)
}

func TestNormalizeTags_Empty(t *testing.T) {
	out, err := NormalizeTags(nil, 10, 30)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestNormalizeTags_TooManyTags(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	_, err := NormalizeTags(in, 10, 30)
	assert.ErrorIs(t, err, ErrTooManyTags)
}

func TestNormalizeTags_TagTooLong(t *testing.T) {
	long := "this-is-a-very-long-tag-that-exceeds-thirty"
	_, err := NormalizeTags([]string{long}, 10, 30)
	assert.ErrorIs(t, err, ErrTagTooLong)
}

func TestNormalizeTags_InvalidChars(t *testing.T) {
	_, err := NormalizeTags([]string{"hello!world"}, 10, 30)
	assert.ErrorIs(t, err, ErrInvalidTag)
}

func TestNormalizeTags_DropsEmptyAfterTrim(t *testing.T) {
	out, err := NormalizeTags([]string{"tech", "   ", ""}, 10, 30)
	require.NoError(t, err)
	assert.Equal(t, []string{"tech"}, out)
}
