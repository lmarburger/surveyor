package surveyor

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/exp/maps"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

var totalChannels = 16
var totalDataSources = 5

type SignalDatum struct {
	// TODO: Remove strings after fully moved to prom
	Frequency, SNRatio, PowerLevel, Correctable, Uncorrectable      string
	IFrequency, ISNRatio, IPowerLevel, ICorrectable, IUncorrectable int
}

type SignalData map[int]SignalDatum

type GraphDetails struct {
	outputPath, title, verticalLabel string
	start, end                       time.Time
	width, height                    int
}

// Assuming graphs will be 1,000 pixels wide, the raw data at 5s resolution could render up to 83m of data. Larger than
// that, it would start aggregating points. 1 pixel per data point is excessive. I'd rather have each be at least 2
// pixels for readability. That would reduce the number of points on the graph to 500 which is a 42m window at 5s.
//
// The use case for storing data at 5s granularity is monitoring an active issue to get immediate feedback. For anything
// else, 5s is overkill.
//
// I want to be able to draw the graphs below. The time period is the amount of time represented by each data point if
// the graph contained 500 data points.
//
//   1h:  7.2s
//   3h: 21.6s
//  30h:  3.6m
//  10d: 28.8m
//  30d: 86.4m
// 400d: 19.2h
//
// Saying that a graph of 400d doesn't need resolution higher than 19.2h isn't very relevant because I'd want to produce
// graphs of smaller time windows. The real question is what views do I want available when looking at historical data?
// For that data, let's say a 30d resolution be sufficient. In that case, here's the aggregation windows that would be
// necessary.
//
// RRA:AVERAGE:0.5:1:180    -  15m @  5s
// RRA:AVERAGE:0.5:4:540    -   3h @ 20s
// RRA:AVERAGE:0.5:60:360   -  30h @  5m
// RRA:AVERAGE:0.5:360:480  -  10d @ 30m
// RRA:AVERAGE:0.5:720:9600 - 400d @  1h
//
// That's 11,160 data points. With 5 data sources and storing avg, min, and max, that creates a 21MB database. 💯

func CreateRRD(ctx context.Context, path string, step, heartbeat time.Duration) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return nil
	}

	stepSeconds := int(step.Seconds())
	heartbeatSeconds := int(heartbeat.Seconds())

	var stderr bytes.Buffer
	args := []string{"create", path, "--step", strconv.Itoa(stepSeconds)}

	startLength := len(args)
	args = append(args, dataSources("frequency", "GAUGE", heartbeatSeconds, "U", "U")...)
	args = append(args, dataSources("snratio", "GAUGE", heartbeatSeconds, "U", "U")...)
	args = append(args, dataSources("powerlevel", "GAUGE", heartbeatSeconds, "U", "U")...)
	args = append(args, dataSources("correctable", "COUNTER", heartbeatSeconds, "0", "U")...)
	args = append(args, dataSources("uncorrectable", "COUNTER", heartbeatSeconds, "0", "U")...)
	if err := assertExpectedDataSources(len(args) - startLength); err != nil {
		return err
	}

	args = append(args, "RRA:AVERAGE:0.5:1:180")
	args = append(args, "RRA:MIN:0.5:1:180")
	args = append(args, "RRA:MAX:0.5:1:180")

	args = append(args, "RRA:AVERAGE:0.5:4:540")
	args = append(args, "RRA:MIN:0.5:4:540")
	args = append(args, "RRA:MAX:0.5:4:540")

	args = append(args, "RRA:AVERAGE:0.5:60:360")
	args = append(args, "RRA:MIN:0.5:60:360")
	args = append(args, "RRA:MAX:0.5:60:360")

	args = append(args, "RRA:AVERAGE:0.5:360:480")
	args = append(args, "RRA:MIN:0.5:360:480")
	args = append(args, "RRA:MAX:0.5:360:480")

	args = append(args, "RRA:AVERAGE:0.5:720:9600")
	args = append(args, "RRA:MIN:0.5:720:9600")
	args = append(args, "RRA:MAX:0.5:720:9600")

	cmd := exec.CommandContext(ctx, "rrdtool", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating database: %w\n%v", err, stderr.String())
	}

	return err
}

func WriteRRD(ctx context.Context, path string, start time.Time, data SignalData) error {
	flattened := flattenChannelData(data)
	parts := make([]string, 0, len(flattened)+1)
	parts = append(parts, strconv.FormatInt(start.Unix(), 10))
	parts = append(parts, flattened...)
	joined := strings.Join(parts, ":")
	fmt.Printf("write: %v\n", joined)

	var stderr bytes.Buffer
	args := []string{"update", path, joined}
	cmd := exec.CommandContext(ctx, "rrdtool", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error updating database: %w\n%v\n%v", err, args, stderr.String())
	}
	return nil
}

func flattenChannelData(data SignalData) []string {
	sortedChannelIDs := maps.Keys(data)
	slices.Sort(sortedChannelIDs)

	flattened := make([]string, totalChannels*totalDataSources)
	for i, channelID := range sortedChannelIDs {
		values := signalDatumToSlice(data[channelID])
		for j := range len(values) {
			next := j * totalChannels
			flattened[i+next] = values[j]
		}
	}

	lenChannels := len(sortedChannelIDs)
	for i := range totalChannels - lenChannels {
		base := i + lenChannels
		for j := range totalDataSources {
			next := j * totalChannels
			flattened[base+next] = "U"
		}
	}

	return flattened
}

func assertExpectedDataSources(length int) error {
	expected := totalChannels * totalDataSources
	if expected == length {
		return nil
	}

	return fmt.Errorf("unexpected number of data sources, expected %d got %d\n", expected, length)
}
