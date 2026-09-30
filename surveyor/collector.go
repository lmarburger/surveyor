package surveyor

import (
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	downstreamLabels = []string{"channel_id", "frequency_mhz", "modulation"}
	upstreamLabels   = []string{"channel_id", "frequency_mhz", "type"}

	modemUp = prometheus.NewDesc("surveyor_modem_up",
		"1 if the most recent modem poll succeeded, 0 otherwise.", nil, nil)
	consecutiveFailures = prometheus.NewDesc("surveyor_poll_consecutive_failures",
		"Modem polls that have failed in a row since the last success.", nil, nil)
	lastSuccess = prometheus.NewDesc("surveyor_last_success_timestamp_seconds",
		"Unix time of the most recent successful modem poll.", nil, nil)
	modemInfo = prometheus.NewDesc("surveyor_modem_info",
		"Modem software versions, as labels. Always 1.", []string{"docsis", "hardware", "firmware"}, nil)

	dsFrequency = prometheus.NewDesc("surveyor_downstream_frequency_hertz",
		"Center frequency of a downstream channel.", downstreamLabels, nil)
	dsLocked = prometheus.NewDesc("surveyor_downstream_locked",
		"1 if the modem reports the downstream channel as locked.", downstreamLabels, nil)
	dsPower = prometheus.NewDesc("surveyor_downstream_power_dbmv",
		"Receive power of a downstream channel.", downstreamLabels, nil)
	dsSNR = prometheus.NewDesc("surveyor_downstream_snr_db",
		"Signal to noise ratio of a downstream channel.", downstreamLabels, nil)
	dsCorrected = prometheus.NewDesc("surveyor_downstream_corrected_codewords_total",
		"Codewords received with errors that were corrected.", downstreamLabels, nil)
	dsUncorrectable = prometheus.NewDesc("surveyor_downstream_uncorrectable_codewords_total",
		"Codewords received with errors that could not be corrected.", downstreamLabels, nil)

	usFrequency = prometheus.NewDesc("surveyor_upstream_frequency_hertz",
		"Center frequency of an upstream channel.", upstreamLabels, nil)
	usLocked = prometheus.NewDesc("surveyor_upstream_locked",
		"1 if the modem reports the upstream channel as locked.", upstreamLabels, nil)
	usPower = prometheus.NewDesc("surveyor_upstream_power_dbmv",
		"Transmit power of an upstream channel.", upstreamLabels, nil)
	usSymbolRate = prometheus.NewDesc("surveyor_upstream_symbol_rate",
		"Symbol rate of an upstream channel, in symbols per second.", upstreamLabels, nil)
)

// Collector reports the poller's latest snapshot. It never talks to the modem,
// so a scrape is instant no matter how the modem is doing.
type Collector struct {
	poller *Poller
	// Channel data older than this is withheld rather than reported, so an
	// unreachable modem shows as a gap instead of a flat line.
	staleAfter time.Duration
	now        func() time.Time
}

func NewCollector(poller *Poller, staleAfter time.Duration) *Collector {
	return &Collector{poller: poller, staleAfter: staleAfter, now: time.Now}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		modemUp, consecutiveFailures, lastSuccess, modemInfo,
		dsFrequency, dsLocked, dsPower, dsSNR, dsCorrected, dsUncorrectable,
		usFrequency, usLocked, usPower, usSymbolRate,
	} {
		ch <- d
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	snap := c.poller.Snapshot()

	ch <- prometheus.MustNewConstMetric(modemUp, prometheus.GaugeValue, boolValue(snap.Up))
	ch <- prometheus.MustNewConstMetric(consecutiveFailures, prometheus.GaugeValue, float64(snap.ConsecutiveFailures))

	if snap.LastSuccess.IsZero() {
		return
	}
	ch <- prometheus.MustNewConstMetric(lastSuccess, prometheus.GaugeValue, float64(snap.LastSuccess.UnixNano())/1e9)

	if c.now().Sub(snap.LastSuccess) > c.staleAfter {
		return
	}

	status := snap.Status
	if status.Info.Firmware != "" {
		ch <- prometheus.MustNewConstMetric(modemInfo, prometheus.GaugeValue, 1,
			status.Info.DOCSIS, status.Info.Hardware, status.Info.Firmware)
	}

	// The registry fails the whole scrape on a duplicate series, so a modem
	// that repeats a record must not take every other metric down with it.
	seen := make(map[string]bool)
	firstTime := func(direction string, labels []string) bool {
		key := direction + "\xff" + strings.Join(labels, "\xff")
		if seen[key] {
			return false
		}
		seen[key] = true
		return true
	}

	for _, d := range status.Downstream {
		labels := []string{strconv.Itoa(d.ChannelID), mhz(d.FrequencyHz), d.Modulation}
		if !firstTime("down", labels) {
			continue
		}
		ch <- prometheus.MustNewConstMetric(dsFrequency, prometheus.GaugeValue, d.FrequencyHz, labels...)
		ch <- prometheus.MustNewConstMetric(dsLocked, prometheus.GaugeValue, boolValue(d.Locked), labels...)
		ch <- prometheus.MustNewConstMetric(dsPower, prometheus.GaugeValue, d.PowerDBmV, labels...)
		ch <- prometheus.MustNewConstMetric(dsSNR, prometheus.GaugeValue, d.SNRDB, labels...)
		ch <- prometheus.MustNewConstMetric(dsCorrected, prometheus.CounterValue, d.Corrected, labels...)
		ch <- prometheus.MustNewConstMetric(dsUncorrectable, prometheus.CounterValue, d.Uncorrectable, labels...)
	}

	for _, u := range status.Upstream {
		labels := []string{strconv.Itoa(u.ChannelID), mhz(u.FrequencyHz), u.Type}
		if !firstTime("up", labels) {
			continue
		}
		ch <- prometheus.MustNewConstMetric(usFrequency, prometheus.GaugeValue, u.FrequencyHz, labels...)
		ch <- prometheus.MustNewConstMetric(usLocked, prometheus.GaugeValue, boolValue(u.Locked), labels...)
		ch <- prometheus.MustNewConstMetric(usPower, prometheus.GaugeValue, u.PowerDBmV, labels...)
		ch <- prometheus.MustNewConstMetric(usSymbolRate, prometheus.GaugeValue, u.SymbolRate, labels...)
	}
}

func mhz(hz float64) string {
	return strconv.FormatFloat(hz/1e6, 'f', -1, 64)
}

func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
