package surveyor

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParsePath(t *testing.T) {
	t.Run("parses path", func(t *testing.T) {
		details, err := parsePath("/snratio-15m-900x200.png")
		assert.Nil(t, err)
		assert.Equal(t, "snratio", details.data)
		assert.Equal(t, time.Minute*15, details.duration)
		assert.Equal(t, 900, details.width)
		assert.Equal(t, 200, details.height)
	})

	t.Run("returns error with invalid path", func(t *testing.T) {
		details, err := parsePath("/invalid.png")
		assert.Empty(t, details)
		assert.ErrorContains(t, err, "invalid path format \"/invalid.png\"")
	})

	t.Run("returns error without .png suffix", func(t *testing.T) {
		details, err := parsePath("/snratio-15m-900x200.lol")
		assert.Empty(t, details)
		assert.ErrorContains(t, err, "invalid format \"/snratio-15m-900x200.lol\", expected \".png\"")
	})

	t.Run("returns error with invalid width", func(t *testing.T) {
		details, err := parsePath("/snratio-15m-lolx200.png")
		assert.Empty(t, details)
		assert.ErrorContains(t, err, "invalid width \"lol\"")
	})

	t.Run("returns error with invalid height", func(t *testing.T) {
		details, err := parsePath("/snratio-15m-900xlol.png")
		assert.Empty(t, details)
		assert.ErrorContains(t, err, "invalid height \"lol\"")
	})
}
