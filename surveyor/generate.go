package surveyor

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/exp/maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	Median = iota
	Stacked
	Breakout
)

var allGraphs = map[string]struct {
	title, label string
	graphType    int
}{
	"snratio":       {"Signal / Noise Ratio", "dB", Median},
	"powerlevel":    {"Power Level", "dBmV", Median},
	"frequency":     {"Channel Frequencies", "Hz", Breakout},
	"unerrored":     {"Unerrored Codewords", "count/sec", Median},
	"correctable":   {"Correctable Codewords by Channel", "count/sec", Stacked},
	"uncorrectable": {"Unorrectable Codewords by Channel", "count/sec", Stacked},
}

func WriteGraph(ctx context.Context, rrdPath, path string, render RenderDetails) error {
	end := time.Now()
	start := end.Add(-render.duration)
	graph := allGraphs[render.data]
	details := GraphDetails{
		outputPath:    path,
		title:         graph.title,
		verticalLabel: graph.label,
		start:         start,
		end:           end,
		width:         render.width,
		height:        render.height,
	}

	var err error
	switch graph.graphType {
	case Median:
		err = graphMedian(ctx, rrdPath, render.data, details)
	case Stacked:
		err = graphStackedBreakout(ctx, rrdPath, render.data, details)
	default:
		err = graphBreakout(ctx, rrdPath, render.data, details)
	}
	if err != nil {
		return fmt.Errorf("error building %s graph: %w", graph.title, err)
	}

	return nil
}

func dataSources(name string, sourceType string, heartbeat int, min, max string) []string {
	var args []string
	for i := 0; i < totalChannels; i++ {
		args = append(args, fmt.Sprintf("DS:%s%d:%s:%d:%s:%s", name, i, sourceType, heartbeat, min, max))
	}
	return args
}

func graphMedian(ctx context.Context, rrdPath, def string, graph GraphDetails) error {
	args := graphCommand(graph)
	args = append(args, dataDefinitions(rrdPath, def, totalChannels)...)
	args = append(args,
		aggregation("min", "SMIN", totalChannels),
		aggregation("max", "SMAX", totalChannels),
		aggregation("med", "MEDIAN", totalChannels),
		"AREA:max#CEDCE8:Min / Max",
		"AREA:min#FFFFFF",
		"LINE2:med#0E4D8A:Median",
	)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rrdtool", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error drawing graph: %w\n%v\n%v", err, args, stderr.String())
	}
	return nil
}

func graphBreakout(ctx context.Context, rrdPath, def string, graph GraphDetails) error {
	args := graphCommand(graph)
	args = append(args, "--no-legend")
	args = append(args, dataDefinitions(rrdPath, def, totalChannels)...)
	args = append(args, lines(totalChannels)...)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rrdtool", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error drawing graph: %w\n%v\n%v", err, args, stderr.String())
	}
	return nil
}

func graphStackedBreakout(ctx context.Context, rrdPath, def string, graph GraphDetails) error {
	args := graphCommand(graph)
	args = append(args, "--no-legend")
	args = append(args, dataDefinitions(rrdPath, def, totalChannels)...)
	args = append(args, stack(totalChannels)...)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rrdtool", args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error drawing graph: %w\n%v\n%v", err, args, stderr.String())
	}
	return nil
}

func graphCommand(graph GraphDetails) []string {
	return []string{
		"graph", graph.outputPath,
		"--start", strconv.FormatInt(graph.start.Unix(), 10),
		"--end", strconv.FormatInt(graph.end.Unix(), 10),
		"--width", strconv.Itoa(graph.width),
		"--height", strconv.Itoa(graph.height),
		"--title", graph.title,
		"--vertical-label", graph.verticalLabel,
	}
}

func dataDefinitions(rrdPath, prefix string, size int) []string {
	defs := make([]string, 0, size)
	for i := 0; i < size; i++ {
		defs = append(defs, fmt.Sprintf("DEF:data%d=%s:%s%d:AVERAGE", i, rrdPath, prefix, i))
	}
	return defs
}

func aggregation(name, agg string, size int) string {
	dataNames := make([]string, 0, size)
	for i := 0; i < size; i++ {
		dataNames = append(dataNames, fmt.Sprintf("data%d", i))
	}
	return fmt.Sprintf("CDEF:%s=%s,%d,%s", name, strings.Join(dataNames, ","), len(dataNames), agg)
}

var colors = []string{
	"#E51616",
	"#E5B116",
	"#7EE516",
	"#16E54A",
	"#16E5E5",
	"#164AE5",
	"#7E16E5",
	"#E516B1",
}

func lines(size int) []string {
	var items = make([]string, 0, size)
	for i := 0; i < size; i++ {
		items = append(items, fmt.Sprintf("LINE:data%d%s:%d", i, colors[i%8], i+1))
	}
	return items
}

func stack(size int) []string {
	var areas = make([]string, 0, size)
	areas = append(areas, fmt.Sprintf("AREA:data0%s:1", colors[0]))
	for i := 1; i < size; i++ {
		areas = append(areas, fmt.Sprintf("AREA:data%d%s:%d:STACK", i, colors[i%8], i+1))
	}
	return areas
}

func flattenChannelData(data SignalData) []string {
	channelsCount := len(data)
	sortedChannelIDs := maps.Keys(data)
	slices.Sort(sortedChannelIDs)

	flattened := make([]string, channelsCount*6)
	for i, channelID := range sortedChannelIDs {
		values := signalDatumToSlice(data[channelID])
		for j := range len(values) {
			next := j * channelsCount
			flattened[i+next] = values[j]
		}
	}

	return flattened
}

func signalDatumToSlice(datum SignalDatum) []string {
	return []string{datum.Frequency, datum.SNRatio, datum.PowerLevel, datum.Unerrored, datum.Correctable, datum.Uncorrectable}
}
