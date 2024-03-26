package surveyor

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChannelInfosToSignalData(t *testing.T) {
	t.Run("returns signal data", func(t *testing.T) {
		infos := `1^Locked^QAM256^3^723000000^7^29^53513438^417213^|+|2^Locked^QAM256^1^651000000^7^31^2895843^340003^|+|3^Locked^QAM256^2^657000000^8^31^3946112^232351^`
		data, err := ChannelInfosToSignalData(infos)
		require.Nil(t, err)

		expected := SignalData{
			1: SignalDatum{
				Frequency:     651000000,
				SNRatio:       31,
				PowerLevel:    7,
				Correctable:   2895843,
				Uncorrectable: 340003,
			},
			2: SignalDatum{
				Frequency:     657000000,
				SNRatio:       31,
				PowerLevel:    8,
				Correctable:   3946112,
				Uncorrectable: 232351,
			},
			3: SignalDatum{
				Frequency:     723000000,
				SNRatio:       29,
				PowerLevel:    7,
				Correctable:   53513438,
				Uncorrectable: 417213,
			},
		}

		assert.Equal(t, expected, data)
	})

	t.Run("returns signal data without trailing ^", func(t *testing.T) {
		infos := `1^Locked^QAM256^3^723000000^7^29^53513438^417213`
		data, err := ChannelInfosToSignalData(infos)
		require.Nil(t, err)

		expected := SignalData{
			3: SignalDatum{
				Frequency:     723000000,
				SNRatio:       29,
				PowerLevel:    7,
				Correctable:   53513438,
				Uncorrectable: 417213,
			},
		}

		assert.Equal(t, expected, data)
	})

	t.Run("returns error with missing fields", func(t *testing.T) {
		infos := `1^Locked^QAM256^3^723000000^7^29^53513438`
		data, err := ChannelInfosToSignalData(infos)
		assert.NotNil(t, err)
		assert.Nil(t, data)
	})

	t.Run("returns error with too many fields", func(t *testing.T) {
		infos := `1^Locked^QAM256^3^723000000^7^29^53513438^417213^42`
		data, err := ChannelInfosToSignalData(infos)
		assert.NotNil(t, err)
		assert.Nil(t, data)
	})

	t.Run("returns error with invalid channel id", func(t *testing.T) {
		infos := `1^Locked^QAM256^FAIL^723000000^7^29^53513438^417213`
		data, err := ChannelInfosToSignalData(infos)
		assert.NotNil(t, err)
		assert.Nil(t, data)
	})
}
