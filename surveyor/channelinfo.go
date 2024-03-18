package surveyor

import (
	"fmt"
	"strconv"
	"strings"
)

type ModemStatus struct {
	Index         int
	LockStatus    string
	Modulation    string
	Frequency     int
	Power         int
	SNRorMER      int
	Corrected     int
	Uncorrectable int
}

// `1^Locked^QAM256^13^723000000^7^29^53513438^417213^|+|2^Locked^QAM256^1^651000000^7^31^2895843^340003^|+|3^Locked^QAM256^2^657000000^8^31^3946112^232351^`

func ChannelInfosToSignalData(channels string) (SignalData, error) {
	data := make(SignalData)
	records := strings.Split(channels, "|+|")
	for _, record := range records {
		record = strings.TrimSuffix(record, "^")
		fields := strings.Split(record, "^")

		if len(fields) != 9 {
			return nil, fmt.Errorf("invalid record, expected 9 fields, got %d: %q", len(fields), record)
		}

		channelID, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("error parsing channel id: %w", err)
		}

		data[channelID] = SignalDatum{
			Frequency:     fields[4],
			SNRatio:       fields[6],
			PowerLevel:    fields[5],
			Correctable:   fields[7],
			Uncorrectable: fields[8],
		}
	}

	return data, nil
}
