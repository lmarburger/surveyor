package surveyor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCreateRRD(t *testing.T) {
	t.Run("rrd created without error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)

		path := "test.rrd"
		err := CreateRRD(ctx, path, time.Second, time.Second*2)
		require.Nilf(t, err, "error creating rrd: %v", err)
		assert.FileExistsf(t, path, "created rrd not found: %v", err)
	})
}

func TestWriteRRD(t *testing.T) {
	t.Run("write to rrd without error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		data := SignalData{
			7:  {"71", "72", "73", "74", "75", "76"},
			4:  {"41", "42", "43", "44", "45", "46"},
			5:  {"51", "52", "53", "54", "55", "56"},
			12: {"121", "122", "123", "124", "125", "126"},
			6:  {"61", "62", "63", "64", "65", "66"},
			8:  {"81", "82", "83", "84", "85", "86"},
			11: {"111", "112", "113", "114", "115", "116"},
			13: {"131", "132", "133", "134", "135", "136"},
		}
		require.Equal(t, totalChannels, len(data))

		err := WriteRRD(ctx, "test.rrd", time.Now().Add(time.Second*2), data)
		assert.Nilf(t, err, "error writing to rrd: %v", err)
	})
}
