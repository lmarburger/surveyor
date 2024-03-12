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

	t.Run("write to rrd without error with fewer than expected channels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		data := SignalData{
			7:  {"71", "72", "73", "74", "75", "76"},
			4:  {"41", "42", "43", "44", "45", "46"},
			5:  {"51", "52", "53", "54", "55", "56"},
			11: {"111", "112", "113", "114", "115", "116"},
			13: {"131", "132", "133", "134", "135", "136"},
		}

		err := WriteRRD(ctx, "test.rrd", time.Now().Add(time.Second*2), data)
		assert.Nilf(t, err, "error writing to rrd: %v", err)
	})
}

func TestFlattenChannelData(t *testing.T) {
	t.Run("returns a list of data grouped by value sorted by channel", func(t *testing.T) {
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

		expected := []string{
			"41", "51", "61", "71", "81", "111", "121", "131",
			"42", "52", "62", "72", "82", "112", "122", "132",
			"43", "53", "63", "73", "83", "113", "123", "133",
			"44", "54", "64", "74", "84", "114", "124", "134",
			"45", "55", "65", "75", "85", "115", "125", "135",
			"46", "56", "66", "76", "86", "116", "126", "136",
		}

		actual := flattenChannelData(data)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns a list of data grouped by value sorted by channel with fewer than expected channels", func(t *testing.T) {
		data := SignalData{
			4:  {"41", "42", "43", "44", "45", "46"},
			6:  {"61", "62", "63", "64", "65", "66"},
			13: {"131", "132", "133", "134", "135", "136"},
			2:  {"21", "22", "23", "24", "25", "26"},
		}
		expected := []string{
			"21", "41", "61", "131", "U", "U", "U", "U",
			"22", "42", "62", "132", "U", "U", "U", "U",
			"23", "43", "63", "133", "U", "U", "U", "U",
			"24", "44", "64", "134", "U", "U", "U", "U",
			"25", "45", "65", "135", "U", "U", "U", "U",
			"26", "46", "66", "136", "U", "U", "U", "U",
		}

		actual := flattenChannelData(data)
		assert.Equal(t, expected, actual)
	})
}
