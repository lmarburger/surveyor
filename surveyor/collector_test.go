package surveyor

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCollector(snap Snapshot, now time.Time) *Collector {
	p := newTestPoller(&scriptedFetcher{results: []fetchResult{{}}})
	p.snap = snap
	c := NewCollector(p, 90*time.Second)
	c.now = func() time.Time { return now }
	return c
}

var lastPoll = time.Unix(1790000000, 0)

var sampleStatus = Status{
	Info: ModemInfo{DOCSIS: "DOCSIS 3.1", Hardware: "V1.0", Firmware: "TB01.01.001.14"},
	Downstream: []DownstreamChannel{
		{ChannelID: 32, Locked: true, Modulation: "QAM256", FrequencyHz: 741000000, PowerDBmV: 2, SNRDB: 34, Corrected: 39579},
		{ChannelID: 1, Locked: true, Modulation: "Unknown", FrequencyHz: 555000000, PowerDBmV: -47},
	},
	Upstream: []UpstreamChannel{
		{ChannelID: 13, Locked: true, Type: "SC-QAM", SymbolRate: 6400000, FrequencyHz: 37800000, PowerDBmV: 38.8},
	},
}

func TestCollector(t *testing.T) {
	t.Run("reports the modem down before the first success", func(t *testing.T) {
		c := newTestCollector(Snapshot{ConsecutiveFailures: 3}, lastPoll)

		err := testutil.CollectAndCompare(c, strings.NewReader(`
# HELP surveyor_modem_up 1 if the most recent modem poll succeeded, 0 otherwise.
# TYPE surveyor_modem_up gauge
surveyor_modem_up 0
# HELP surveyor_poll_consecutive_failures Modem polls that have failed in a row since the last success.
# TYPE surveyor_poll_consecutive_failures gauge
surveyor_poll_consecutive_failures 3
`))
		require.NoError(t, err)
	})

	t.Run("reports channels from a fresh snapshot", func(t *testing.T) {
		c := newTestCollector(Snapshot{Status: sampleStatus, Up: true, LastSuccess: lastPoll}, lastPoll.Add(30*time.Second))

		err := testutil.CollectAndCompare(c, strings.NewReader(`
# HELP surveyor_downstream_snr_db Signal to noise ratio of a downstream channel.
# TYPE surveyor_downstream_snr_db gauge
surveyor_downstream_snr_db{channel_id="1",frequency_mhz="555",modulation="Unknown"} 0
surveyor_downstream_snr_db{channel_id="32",frequency_mhz="741",modulation="QAM256"} 34
# HELP surveyor_upstream_power_dbmv Transmit power of an upstream channel.
# TYPE surveyor_upstream_power_dbmv gauge
surveyor_upstream_power_dbmv{channel_id="13",frequency_mhz="37.8",type="SC-QAM"} 38.8
# HELP surveyor_modem_info Modem software versions, as labels. Always 1.
# TYPE surveyor_modem_info gauge
surveyor_modem_info{docsis="DOCSIS 3.1",firmware="TB01.01.001.14",hardware="V1.0"} 1
# HELP surveyor_last_success_timestamp_seconds Unix time of the most recent successful modem poll.
# TYPE surveyor_last_success_timestamp_seconds gauge
surveyor_last_success_timestamp_seconds 1.79e+09
`), "surveyor_downstream_snr_db", "surveyor_upstream_power_dbmv", "surveyor_modem_info", "surveyor_last_success_timestamp_seconds")
		require.NoError(t, err)
		assert.Equal(t, 2, testutil.CollectAndCount(c, "surveyor_downstream_uncorrectable_codewords_total"))
	})

	t.Run("withholds channels from a stale snapshot", func(t *testing.T) {
		c := newTestCollector(Snapshot{Status: sampleStatus, LastSuccess: lastPoll, ConsecutiveFailures: 5}, lastPoll.Add(10*time.Minute))

		assert.Zero(t, testutil.CollectAndCount(c, "surveyor_downstream_snr_db"))
		assert.Zero(t, testutil.CollectAndCount(c, "surveyor_modem_info"))
		assert.Equal(t, 1, testutil.CollectAndCount(c, "surveyor_last_success_timestamp_seconds"))
	})

	t.Run("drops duplicate records instead of failing the scrape", func(t *testing.T) {
		status := sampleStatus
		status.Downstream = append(status.Downstream, status.Downstream[0])
		c := newTestCollector(Snapshot{Status: status, Up: true, LastSuccess: lastPoll}, lastPoll)

		assert.Equal(t, 2, testutil.CollectAndCount(c, "surveyor_downstream_snr_db"))
	})
}
