package surveyor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	Median = iota
	Stacked
	Breakout
)

var simpleRainbow = []string{
	"#E51616",
	"#E5B116",
	"#7EE516",
	"#16E54A",
	"#16E5E5",
	"#164AE5",
	"#7E16E5",
	"#E516B1",
}

var rainbow = []string{
	"#D32F2F",
	"#E64A19",
	"#F57C00",
	"#FBC02D",
	"#AFB42B",
	"#7CB342",
	"#388E3C",
	"#009688",
	"#039BE5",
	"#1976D2",
	"#303F9F",
	"#512DA8",
	"#7B1FA2",
	"#C2185B",
	"#E91E63",
	"#D81B60",
}

var colors = []string{
	"#FF0000",
	"#008000",
	"#0000FF",
	"#FFFF00",
	"#FFA500",
	"#800080",
	"#00FFFF",
	"#FF00FF",
	"#00FF00",
	"#FFC0CB",
	"#008080",
	"#E6E6FA",
	"#808000",
	"#800000",
	"#000080",
	"#808080",
}

var pastels = []string{
	"#E57373",
	"#81C784",
	"#64B5F6",
	"#FFF176",
	"#FFB74D",
	"#BA68C8",
	"#4DD0E1",
	"#F06292",
	"#DCE775",
	"#F8BBD0",
	"#4DB6AC",
	"#B39DDB",
	"#FFCCBC",
	"#FF8A65",
	"#4FC3F7",
	"#AED581",
}

var percentileColors = map[string]string{
	"100p":   "#CEDCE8",
	"75p":    "#8EACC9",
	"50p":    "#4E7DA9",
	"median": "#0E4D8A",
}

var allGraphs = map[string]struct {
	title, label string
	graphType    int
}{
	"snratio":       {"Signal / Noise Ratio", "dB", Median},
	"powerlevel":    {"Power Level", "dBmV", Median},
	"frequency":     {"Channel Frequencies", "Hz", Breakout},
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
	list := dataList("data", totalChannels)
	args = append(args,
		fmt.Sprintf("CDEF:median=%s,%d,MEDIAN", list, totalChannels),
		fmt.Sprintf("CDEF:min100p=%s,%d,SMIN", list, totalChannels),
		fmt.Sprintf("CDEF:max100p=%s,%d,SMAX", list, totalChannels),
		fmt.Sprintf("CDEF:min75p=%s,%d,SORT,%d,REV,POP,POP,%d,SMIN", list, totalChannels, totalChannels, totalChannels-2),
		fmt.Sprintf("CDEF:max75p=%s,%d,SORT,POP,POP,%d,SMAX", list, totalChannels, totalChannels-2),
		fmt.Sprintf("CDEF:min50p=%s,%d,SORT,%d,REV,POP,POP,POP,POP,%d,SMIN", list, totalChannels, totalChannels, totalChannels-4),
		fmt.Sprintf("CDEF:max50p=%s,%d,SORT,POP,POP,POP,POP,%d,SMAX", list, totalChannels, totalChannels-4),
		"CDEF:range100p=max100p,min100p,-",
		"CDEF:range75p=max75p,min75p,-",
		"CDEF:range50p=max50p,min50p,-",
		"LINE:min100p",
		fmt.Sprintf("AREA:range100p%s:100p:STACK", percentileColors["100p"]),
		"LINE:min75p",
		fmt.Sprintf("AREA:range75p%s:75p:STACK", percentileColors["75p"]),
		"LINE:min50p",
		fmt.Sprintf("AREA:range50p%s:50p:STACK", percentileColors["50p"]),
		fmt.Sprintf("LINE2:median%s:Median", percentileColors["median"]),
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
	//args = append(args, "--no-legend")
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
	//args = append(args, "--no-legend")
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

func dataList(name string, size int) string {
	list := make([]string, 0, size)
	for i := 0; i < size; i++ {
		list = append(list, fmt.Sprintf("%s%d", name, i))
	}
	return strings.Join(list, ",")
}

func lines(size int) []string {
	var items = make([]string, 0, size)
	for i := 0; i < size; i++ {
		items = append(items, fmt.Sprintf("LINE:data%d%s:%d", i, colors[i%len(colors)], i+1))
	}
	return items
}

func stack(size int) []string {
	var areas = make([]string, 0, size)
	areas = append(areas, fmt.Sprintf("AREA:data0%s:1", colors[0]))
	for i := 1; i < size; i++ {
		areas = append(areas, fmt.Sprintf("AREA:data%d%s:%d:STACK", i, colors[i%len(colors)], i+1))
	}
	return areas
}

func signalDatumToSlice(datum SignalDatum) []string {
	return []string{datum.Frequency, datum.SNRatio, datum.PowerLevel, datum.Correctable, datum.Uncorrectable}
}
