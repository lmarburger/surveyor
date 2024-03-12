package surveyor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestGraphMedian(t *testing.T) {
	t.Run("graph median without error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		path := "test.rrd"
		err := graphMedian(ctx, path, "snratio", GraphDetails{
			outputPath:    "out.png",
			title:         "Test Median",
			verticalLabel: "test",
			start:         time.Now(),
			end:           time.Now().Add(time.Minute),
			width:         10,
			height:        10,
		})
		require.Nilf(t, err, "error creating graph: %v", err)
		assert.FileExistsf(t, path, "created graph not found: %v", err)
	})
}

func TestGraphBreakout(t *testing.T) {
	t.Run("graph breakout without error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		useTempDir(t)
		createTestRRD(t, ctx)

		rrdPath := "test.rrd"
		err := graphStackedBreakout(ctx, rrdPath, "correctable", GraphDetails{
			outputPath:    "out.png",
			title:         "Test Breakout",
			verticalLabel: "test",
			start:         time.Now(),
			end:           time.Now().Add(time.Minute),
			width:         10,
			height:        10,
		})
		require.Nilf(t, err, "error creating graph: %v", err)
		assert.FileExistsf(t, rrdPath, "created graph not found: %v", err)
	})
}

func TestGraphCommand(t *testing.T) {
	t.Run("create graph command for given details", func(t *testing.T) {
		start := 1709644493
		end := 709644553
		details := GraphDetails{
			outputPath:    "out.png",
			title:         "Test Breakout",
			verticalLabel: "test",
			start:         time.Unix(int64(start), 0),
			end:           time.Unix(int64(end), 0),
			width:         42,
			height:        24,
		}

		command := graphCommand(details)
		expected := []string{
			"graph", details.outputPath,
			"--start", strconv.Itoa(start),
			"--end", strconv.Itoa(end),
			"--width", strconv.Itoa(details.width),
			"--height", strconv.Itoa(details.height),
			"--title", details.title,
			"--vertical-label", details.verticalLabel,
		}
		assert.Equal(t, expected, command)
	})
}

func TestDataDefinitions(t *testing.T) {
	t.Run("create DEFs of given size", func(t *testing.T) {
		defs := dataDefinitions("test.rrd", "test", 4)
		expected := []string{
			"DEF:data0=test.rrd:test0:AVERAGE",
			"DEF:data1=test.rrd:test1:AVERAGE",
			"DEF:data2=test.rrd:test2:AVERAGE",
			"DEF:data3=test.rrd:test3:AVERAGE",
		}
		assert.Equal(t, expected, defs)
	})
}

func TestDataList(t *testing.T) {
	t.Run("returns list of data items", func(t *testing.T) {
		defs := dataList("data", 4)
		expected := "data0,data1,data2,data3"
		assert.Equal(t, expected, defs)
	})
}

func useTempDir(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	temp := t.TempDir()
	t.Cleanup(func() {
		_ = os.RemoveAll(temp)
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})

	err = os.Chdir(temp)
	if err != nil {
		t.Fatal(err)
	}
}

func createTestRRD(t *testing.T, ctx context.Context) {
	path := "test.rrd"
	err := CreateRRD(ctx, path, time.Second, time.Second*2)
	require.Nilf(t, err, "error creating rrd: %v", err)
	require.FileExistsf(t, path, "created rrd not found: %v", err)
}
