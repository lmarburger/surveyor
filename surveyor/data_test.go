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

var testSignalData = SignalData{
	7:  {"71", "72", "73", "74", "75"},
	4:  {"41", "42", "43", "44", "45"},
	5:  {"51", "52", "53", "54", "55"},
	12: {"121", "122", "123", "124", "125"},
	6:  {"61", "62", "63", "64", "65"},
	8:  {"81", "82", "83", "84", "85"},
	11: {"111", "112", "113", "114", "115"},
	13: {"131", "132", "133", "134", "135"},
	1:  {"11", "12", "13", "14", "15"},
	2:  {"21", "22", "23", "24", "25"},
	3:  {"31", "32", "33", "34", "35"},
	9:  {"91", "92", "93", "94", "95"},
	10: {"101", "102", "103", "104", "105"},
	14: {"141", "142", "143", "144", "145"},
	15: {"151", "152", "153", "154", "155"},
	16: {"161", "162", "163", "164", "165"},
}

func TestWriteRRD(t *testing.T) {
	t.Run("write to rrd without error", func(t *testing.T) {
		require.Equal(t, totalChannels, len(testSignalData))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		err := WriteRRD(ctx, "test.rrd", time.Now().Add(time.Second*2), testSignalData)
		assert.Nilf(t, err, "error writing to rrd: %v", err)
	})

	t.Run("write to rrd without error with fewer than expected channels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		data := SignalData{
			7:  {"71", "72", "73", "74", "75"},
			4:  {"41", "42", "43", "44", "45"},
			5:  {"51", "52", "53", "54", "55"},
			11: {"111", "112", "113", "114", "115"},
			13: {"131", "132", "133", "134", "135"},
		}

		err := WriteRRD(ctx, "test.rrd", time.Now().Add(time.Second*2), data)
		assert.Nilf(t, err, "error writing to rrd: %v", err)
	})
}

func TestFlattenChannelData(t *testing.T) {
	t.Run("returns a list of data grouped by value sorted by channel", func(t *testing.T) {
		require.Equal(t, totalChannels, len(testSignalData))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		expected := []string{
			"11", "21", "31", "41", "51", "61", "71", "81", "91", "101", "111", "121", "131", "141", "151", "161",
			"12", "22", "32", "42", "52", "62", "72", "82", "92", "102", "112", "122", "132", "142", "152", "162",
			"13", "23", "33", "43", "53", "63", "73", "83", "93", "103", "113", "123", "133", "143", "153", "163",
			"14", "24", "34", "44", "54", "64", "74", "84", "94", "104", "114", "124", "134", "144", "154", "164",
			"15", "25", "35", "45", "55", "65", "75", "85", "95", "105", "115", "125", "135", "145", "155", "165",
		}

		actual := flattenChannelData(testSignalData)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns a list of data grouped by value sorted by channel with fewer than expected channels", func(t *testing.T) {
		data := SignalData{
			4:  {"41", "42", "43", "44", "45"},
			6:  {"61", "62", "63", "64", "65"},
			13: {"131", "132", "133", "134", "135"},
			2:  {"21", "22", "23", "24", "25"},
		}
		expected := []string{
			"21", "41", "61", "131", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U",
			"22", "42", "62", "132", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U",
			"23", "43", "63", "133", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U",
			"24", "44", "64", "134", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U",
			"25", "45", "65", "135", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U", "U",
		}

		actual := flattenChannelData(data)
		assert.Equal(t, expected, actual)
	})
}
