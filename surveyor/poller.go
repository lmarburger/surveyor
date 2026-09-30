package surveyor

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Fetcher interface {
	Fetch(ctx context.Context) (Status, error)
}

type Snapshot struct {
	Status              Status
	Up                  bool
	LastSuccess         time.Time
	ConsecutiveFailures int
}

type PollerConfig struct {
	Interval   time.Duration
	Timeout    time.Duration
	MaxBackoff time.Duration
}

// Poller owns all traffic to the modem. Fetches used to happen inside each
// Prometheus scrape, which let every extra reader of /metrics add modem load
// and let a slow modem stall the scrape. Now /metrics only reads the snapshot.
type Poller struct {
	fetcher Fetcher
	config  PollerConfig
	now     func() time.Time

	errors   *prometheus.CounterVec
	skipped  *prometheus.CounterVec
	duration prometheus.Histogram

	mu   sync.RWMutex
	snap Snapshot
}

func NewPoller(fetcher Fetcher, config PollerConfig, reg prometheus.Registerer) *Poller {
	p := &Poller{
		fetcher: fetcher,
		config:  config,
		now:     time.Now,
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "surveyor_poll_errors_total",
			Help: "Failed modem polls, by the stage that failed (login, fetch, parse).",
		}, []string{"stage"}),
		skipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "surveyor_skipped_records_total",
			Help: "Channel records the modem sent that could not be parsed.",
		}, []string{"direction"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "surveyor_poll_duration_seconds",
			Help: "Duration of each modem poll, including any login.",
			// A superset of the old hmac_collect_duration_seconds buckets (the
			// first ten), so heatmaps line up across the rename.
			Buckets: prometheus.ExponentialBuckets(1, 1.25, 14),
		}),
	}
	for _, stage := range []string{StageLogin, StageFetch, StageParse} {
		p.errors.WithLabelValues(stage)
	}
	for _, direction := range []string{"downstream", "upstream"} {
		p.skipped.WithLabelValues(direction)
	}
	reg.MustRegister(p.errors, p.skipped, p.duration)
	return p
}

func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

func (p *Poller) Run(ctx context.Context) {
	for {
		p.Poll(ctx)

		timer := time.NewTimer(p.nextDelay())
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (p *Poller) Poll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	start := p.now()
	status, err := p.fetcher.Fetch(ctx)
	p.duration.Observe(p.now().Sub(start).Seconds())

	if err != nil {
		p.recordFailure(err)
		return
	}

	p.skipped.WithLabelValues("downstream").Add(float64(status.SkippedDownstream))
	p.skipped.WithLabelValues("upstream").Add(float64(status.SkippedUpstream))
	p.recordSuccess(status)
}

func (p *Poller) recordFailure(err error) {
	stage := StageFetch
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) {
		stage = fetchErr.Stage
	}
	p.errors.WithLabelValues(stage).Inc()

	p.mu.Lock()
	p.snap.Up = false
	p.snap.ConsecutiveFailures++
	failures := p.snap.ConsecutiveFailures
	p.mu.Unlock()

	// Log the transition and then periodically, not every poll. A modem that is
	// down for a day would otherwise write thousands of identical lines.
	if failures == 1 || failures%10 == 0 {
		slog.Warn("modem poll failed", "stage", stage, "consecutive_failures", failures,
			"next_attempt_in", p.nextDelay(), "err", err)
	}
}

func (p *Poller) recordSuccess(status Status) {
	p.mu.Lock()
	prev := p.snap
	p.snap = Snapshot{Status: status, Up: true, LastSuccess: p.now()}
	p.mu.Unlock()

	if prev.ConsecutiveFailures > 0 {
		slog.Info("modem poll recovered", "after_failures", prev.ConsecutiveFailures)
	}
	if status.SkippedDownstream+status.SkippedUpstream > 0 {
		slog.Warn("skipped unparseable channel records",
			"downstream", status.SkippedDownstream, "upstream", status.SkippedUpstream)
	}

	if prev.LastSuccess.IsZero() {
		slog.Info("modem poll succeeded", "firmware", status.Info.Firmware,
			"downstream_channels", len(status.Downstream), "upstream_channels", len(status.Upstream))
		return
	}

	// These are the changes that explain a sudden shift in the graphs, and they
	// are otherwise invisible: the ISP pushes them without notice.
	if prev.Status.Info.Firmware != status.Info.Firmware {
		slog.Warn("modem firmware changed", "from", prev.Status.Info.Firmware, "to", status.Info.Firmware)
	}
	if len(prev.Status.Downstream) != len(status.Downstream) || len(prev.Status.Upstream) != len(status.Upstream) {
		slog.Warn("modem channel count changed",
			"downstream_from", len(prev.Status.Downstream), "downstream_to", len(status.Downstream),
			"upstream_from", len(prev.Status.Upstream), "upstream_to", len(status.Upstream))
	}
}

func (p *Poller) nextDelay() time.Duration {
	return backoff(p.config.Interval, p.config.MaxBackoff, p.Snapshot().ConsecutiveFailures)
}

// backoff doubles the wait for each consecutive failure, capped at max. The
// modem keeps working on requests the client has given up on, so retrying at
// the normal rate while it is struggling only deepens the backlog.
func backoff(interval, max time.Duration, failures int) time.Duration {
	delay := interval
	for i := 1; i < failures && delay < max; i++ {
		delay *= 2
	}
	return min(delay, max)
}
