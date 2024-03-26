package surveyor

import (
	"fmt"
	"strconv"
	"strings"
)

// `1^Locked^QAM256^13^723000000^7^29^53513438^417213^|+|2^Locked^QAM256^1^651000000^7^31^2895843^340003^|+|3^Locked^QAM256^2^657000000^8^31^3946112^232351^`
const (
	IDField = iota
	LockStatusField
	ModulationField
	ChannelIDField
	FrequencyField
	PowerLevelField
	SignalNoiseField
	CorrectedField
	UncorrectableField
)

func ChannelInfosToSignalData(channels string) (SignalData, error) {
	data := make(SignalData)
	records := strings.Split(channels, "|+|")
	for _, record := range records {
		record = strings.TrimSuffix(record, "^")
		fields := strings.Split(record, "^")

		if len(fields) != 9 {
			return nil, fmt.Errorf("invalid record, expected 9 fields, got %d: %q", len(fields), record)
		}

		channelID, err := strconv.Atoi(fields[ChannelIDField])
		if err != nil {
			return nil, fmt.Errorf("error parsing channel id: %w", err)
		}

		frequency, err := strconv.Atoi(fields[FrequencyField])
		if err != nil {
			return nil, fmt.Errorf("error parsing frequency: %w", err)
		}

		snratio, err := strconv.Atoi(fields[SignalNoiseField])
		if err != nil {
			return nil, fmt.Errorf("error parsing snratio: %w", err)
		}

		powerLevel, err := strconv.Atoi(fields[PowerLevelField])
		if err != nil {
			return nil, fmt.Errorf("error parsing power level: %w", err)
		}

		correctable, err := strconv.Atoi(fields[CorrectedField])
		if err != nil {
			return nil, fmt.Errorf("error parsing correctable: %w", err)
		}

		uncorrectable, err := strconv.Atoi(fields[UncorrectableField])
		if err != nil {
			return nil, fmt.Errorf("error parsing uncorrectable: %w", err)
		}

		data[channelID] = SignalDatum{
			Frequency:     frequency,
			SNRatio:       snratio,
			PowerLevel:    powerLevel,
			Correctable:   correctable,
			Uncorrectable: uncorrectable,
		}
	}

	return data, nil
}
