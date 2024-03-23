package surveyor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"strconv"
)

var (
	labels         = []string{"channel_id"}
	frequencyGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "frequency",
			Help: "Frequency of a channel",
		},
		labels,
	)
	snRatioGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snratio",
			Help: "Signal / Noise ratio of a channel",
		},
		labels,
	)
	powerLevelGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "power_level",
			Help: "Power level of a channel",
		},
		labels,
	)
	correctableCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "correctable",
			Help: "Total number of correctable codewords",
		},
		labels,
	)
	uncorrectableCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uncorrectable",
			Help: "Total number of uncorrectable codewords",
		},
		labels,
	)
)

func ReportSignalData(data SignalData) error {
	for channel, datum := range data {
		channelID := strconv.Itoa(channel)
		frequencyGauge.WithLabelValues(channelID).Set(float64(datum.IFrequency))
		snRatioGauge.WithLabelValues(channelID).Set(float64(datum.ISNRatio))
		powerLevelGauge.WithLabelValues(channelID).Set(float64(datum.IPowerLevel))
		correctableCounter.WithLabelValues(channelID).Add(float64(datum.ICorrectable))
		uncorrectableCounter.WithLabelValues(channelID).Add(float64(datum.IUncorrectable))
	}
	return nil
}
