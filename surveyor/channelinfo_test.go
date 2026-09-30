package surveyor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDownstream(t *testing.T) {
	t.Run("returns channels", func(t *testing.T) {
		infos := `1^Locked^QAM256^32^741000000^2^34^39579^0^|+|2^Locked^Unknown^1^555000000^-47^0^0^0^`
		channels, skipped := ParseDownstream(infos)

		expected := []DownstreamChannel{
			{ChannelID: 32, Locked: true, Modulation: "QAM256", FrequencyHz: 741000000, PowerDBmV: 2, SNRDB: 34, Corrected: 39579, Uncorrectable: 0},
			{ChannelID: 1, Locked: true, Modulation: "Unknown", FrequencyHz: 555000000, PowerDBmV: -47, SNRDB: 0, Corrected: 0, Uncorrectable: 0},
		}
		assert.Equal(t, expected, channels)
		assert.Zero(t, skipped)
	})

	t.Run("returns channels without trailing ^", func(t *testing.T) {
		channels, skipped := ParseDownstream(`1^Locked^QAM256^3^723000000^7^29^53513438^417213`)

		assert.Equal(t, []DownstreamChannel{
			{ChannelID: 3, Locked: true, Modulation: "QAM256", FrequencyHz: 723000000, PowerDBmV: 7, SNRDB: 29, Corrected: 53513438, Uncorrectable: 417213},
		}, channels)
		assert.Zero(t, skipped)
	})

	t.Run("parses decimal values", func(t *testing.T) {
		channels, _ := ParseDownstream(`1^Not Locked^QAM256^3^723000000^-7.5^29.3^1^2^`)

		assert.Equal(t, -7.5, channels[0].PowerDBmV)
		assert.Equal(t, 29.3, channels[0].SNRDB)
		assert.False(t, channels[0].Locked)
	})

	t.Run("returns nothing for an empty channel list", func(t *testing.T) {
		channels, skipped := ParseDownstream("")
		assert.Empty(t, channels)
		assert.Zero(t, skipped)
	})

	t.Run("skips malformed records and keeps the rest", func(t *testing.T) {
		infos := `1^Locked^QAM256^3^723000000^7^29^53513438^|+|` + // missing a field
			`2^Locked^QAM256^3^723000000^7^29^53513438^417213^42^|+|` + // extra field
			`3^Locked^QAM256^FAIL^723000000^7^29^53513438^417213^|+|` + // bad channel id
			`4^Locked^QAM256^4^729000000^7^29^1^2^`
		channels, skipped := ParseDownstream(infos)

		assert.Equal(t, 3, skipped)
		assert.Len(t, channels, 1)
		assert.Equal(t, 4, channels[0].ChannelID)
	})
}

func TestParseUpstream(t *testing.T) {
	t.Run("returns channels", func(t *testing.T) {
		infos := `1^Locked^SC-QAM^13^6400000^37800000^38.8^|+|4^Locked^SC-QAM^16^3200000^20000000^42.8^`
		channels, skipped := ParseUpstream(infos)

		expected := []UpstreamChannel{
			{ChannelID: 13, Locked: true, Type: "SC-QAM", SymbolRate: 6400000, FrequencyHz: 37800000, PowerDBmV: 38.8},
			{ChannelID: 16, Locked: true, Type: "SC-QAM", SymbolRate: 3200000, FrequencyHz: 20000000, PowerDBmV: 42.8},
		}
		assert.Equal(t, expected, channels)
		assert.Zero(t, skipped)
	})

	t.Run("skips malformed records", func(t *testing.T) {
		channels, skipped := ParseUpstream(`1^Locked^SC-QAM^13^6400000^37800000^|+|2^Locked^SC-QAM^14^6400000^31300000^40.0^`)

		assert.Equal(t, 1, skipped)
		assert.Len(t, channels, 1)
	})
}
