package surveyor

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"strconv"
	"time"
)

var (
	labels        = []string{"channel_id"}
	frequency     = prometheus.NewDesc("frequency", "Frequency of a channel", labels, nil)
	snratio       = prometheus.NewDesc("snratio", "Signal / Noise ratio of a channel", labels, nil)
	powerLevel    = prometheus.NewDesc("power_level", "Power level of a channel", labels, nil)
	correctable   = prometheus.NewDesc("correctable_count", "Total number of correctable codewords", labels, nil)
	uncorrectable = prometheus.NewDesc("uncorrectable_count", "Total number of uncorrectable codewords", labels, nil)

	scrapeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "hmac_collect_duration_seconds",
		Help:    "Duration of HMAC data collection requests in seconds",
		Buckets: prometheus.ExponentialBuckets(1, 1.25, 10),
	})
)

type SignalDataCollector struct {
	client *HNAPClient
}

func NewSignalDataCollector(client *HNAPClient, reg prometheus.Registerer) SignalDataCollector {
	reg.MustRegister(scrapeDuration)
	return SignalDataCollector{client: client}
}

func (this SignalDataCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- frequency
	ch <- snratio
	ch <- powerLevel
	ch <- correctable
	ch <- uncorrectable
}

func (this SignalDataCollector) Collect(ch chan<- prometheus.Metric) {
	// It takes just shy of 3s to get signal data from the modem.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	start := time.Now()
	data, err := this.client.GetSignalData(ctx)
	scrapeDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		log.Printf("error fetching signal data: %v", err)
		return
	}

	for channel, datum := range data {
		channelID := strconv.Itoa(channel)
		ch <- prometheus.MustNewConstMetric(frequency, prometheus.GaugeValue, float64(datum.IFrequency), channelID)
		ch <- prometheus.MustNewConstMetric(snratio, prometheus.GaugeValue, float64(datum.ISNRatio), channelID)
		ch <- prometheus.MustNewConstMetric(powerLevel, prometheus.GaugeValue, float64(datum.IPowerLevel), channelID)
		ch <- prometheus.MustNewConstMetric(correctable, prometheus.CounterValue, float64(datum.ICorrectable), channelID)
		ch <- prometheus.MustNewConstMetric(uncorrectable, prometheus.CounterValue, float64(datum.IUncorrectable), channelID)
	}
}
