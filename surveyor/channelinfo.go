package surveyor

import (
	"fmt"
	"strconv"
	"strings"
)

const recordSeparator = "|+|"

// DownstreamChannel is one record of the modem's downstream list, e.g.
// `1^Locked^QAM256^32^741000000^2^34^39579^0^|+|2^Locked^Unknown^1^555000000^-47^0^0^0^`
// Fields: index, lock status, modulation, channel id, frequency (Hz), power (dBmV),
// SNR (dB), corrected codewords, uncorrectable codewords.
type DownstreamChannel struct {
	ChannelID     int
	Locked        bool
	Modulation    string
	FrequencyHz   float64
	PowerDBmV     float64
	SNRDB         float64
	Corrected     float64
	Uncorrectable float64
}

// UpstreamChannel is one record of the modem's upstream list, e.g.
// `1^Locked^SC-QAM^13^6400000^37800000^38.8^`
// Fields: index, lock status, channel type, channel id, symbol rate, frequency (Hz),
// power (dBmV).
type UpstreamChannel struct {
	ChannelID   int
	Locked      bool
	Type        string
	SymbolRate  float64
	FrequencyHz float64
	PowerDBmV   float64
}

// ParseDownstream returns every well-formed record and a count of the ones it
// had to skip. One garbled channel should not hide the other 31.
func ParseDownstream(channels string) ([]DownstreamChannel, int) {
	var result []DownstreamChannel
	skipped := 0
	for _, fields := range splitRecords(channels) {
		c, err := parseDownstreamRecord(fields)
		if err != nil {
			skipped++
			continue
		}
		result = append(result, c)
	}
	return result, skipped
}

func ParseUpstream(channels string) ([]UpstreamChannel, int) {
	var result []UpstreamChannel
	skipped := 0
	for _, fields := range splitRecords(channels) {
		c, err := parseUpstreamRecord(fields)
		if err != nil {
			skipped++
			continue
		}
		result = append(result, c)
	}
	return result, skipped
}

func splitRecords(channels string) [][]string {
	var records [][]string
	for _, record := range strings.Split(channels, recordSeparator) {
		record = strings.TrimSuffix(strings.TrimSpace(record), "^")
		if record == "" {
			continue
		}
		records = append(records, strings.Split(record, "^"))
	}
	return records
}

func parseDownstreamRecord(fields []string) (DownstreamChannel, error) {
	if len(fields) != 9 {
		return DownstreamChannel{}, fmt.Errorf("expected 9 fields, got %d", len(fields))
	}

	p := fieldParser{fields: fields}
	c := DownstreamChannel{
		Locked:        fields[1] == "Locked",
		Modulation:    fields[2],
		ChannelID:     p.int(3),
		FrequencyHz:   p.float(4),
		PowerDBmV:     p.float(5),
		SNRDB:         p.float(6),
		Corrected:     p.float(7),
		Uncorrectable: p.float(8),
	}
	return c, p.err
}

func parseUpstreamRecord(fields []string) (UpstreamChannel, error) {
	if len(fields) != 7 {
		return UpstreamChannel{}, fmt.Errorf("expected 7 fields, got %d", len(fields))
	}

	p := fieldParser{fields: fields}
	c := UpstreamChannel{
		Locked:      fields[1] == "Locked",
		Type:        fields[2],
		ChannelID:   p.int(3),
		SymbolRate:  p.float(4),
		FrequencyHz: p.float(5),
		PowerDBmV:   p.float(6),
	}
	return c, p.err
}

// fieldParser keeps the first error so a record can be parsed in one expression.
type fieldParser struct {
	fields []string
	err    error
}

func (p *fieldParser) int(i int) int {
	v, err := strconv.Atoi(p.fields[i])
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("field %d: %w", i, err)
	}
	return v
}

func (p *fieldParser) float(i int) float64 {
	v, err := strconv.ParseFloat(p.fields[i], 64)
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("field %d: %w", i, err)
	}
	return v
}
