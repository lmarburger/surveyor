package surveyor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fetchResult struct {
	status Status
	err    error
}

type scriptedFetcher struct {
	results []fetchResult
	calls   int
}

func (f *scriptedFetcher) Fetch(ctx context.Context) (Status, error) {
	r := f.results[min(f.calls, len(f.results)-1)]
	f.calls++
	return r.status, r.err
}

type blockingFetcher struct{}

func (blockingFetcher) Fetch(ctx context.Context) (Status, error) {
	<-ctx.Done()
	return Status{}, &FetchError{Stage: StageFetch, Err: ctx.Err()}
}

func newTestPoller(fetcher Fetcher) *Poller {
	return NewPoller(fetcher, PollerConfig{
		Interval:   30 * time.Second,
		Timeout:    time.Second,
		MaxBackoff: 5 * time.Minute,
	}, prometheus.NewRegistry())
}

func TestBackoff(t *testing.T) {
	for failures, expected := range map[int]time.Duration{
		0:  30 * time.Second,
		1:  30 * time.Second,
		2:  time.Minute,
		3:  2 * time.Minute,
		4:  4 * time.Minute,
		5:  5 * time.Minute,
		50: 5 * time.Minute,
	} {
		assert.Equal(t, expected, backoff(30*time.Second, 5*time.Minute, failures), "failures=%d", failures)
	}
}

func TestPoller_Poll(t *testing.T) {
	ctx := context.Background()
	good := Status{Downstream: []DownstreamChannel{{ChannelID: 1}}}

	t.Run("records a success", func(t *testing.T) {
		p := newTestPoller(&scriptedFetcher{results: []fetchResult{{status: good}}})
		p.Poll(ctx)

		snap := p.Snapshot()
		assert.True(t, snap.Up)
		assert.False(t, snap.LastSuccess.IsZero())
		assert.Equal(t, good, snap.Status)
	})

	t.Run("keeps the last good status through failures and backs off", func(t *testing.T) {
		loginErr := &FetchError{Stage: StageLogin, Err: ErrLoginFailed}
		p := newTestPoller(&scriptedFetcher{results: []fetchResult{{status: good}, {err: loginErr}, {err: loginErr}}})
		p.Poll(ctx)
		lastSuccess := p.Snapshot().LastSuccess
		p.Poll(ctx)
		p.Poll(ctx)

		snap := p.Snapshot()
		assert.False(t, snap.Up)
		assert.Equal(t, 2, snap.ConsecutiveFailures)
		assert.Equal(t, lastSuccess, snap.LastSuccess)
		assert.Equal(t, good, snap.Status)
		assert.Equal(t, time.Minute, p.nextDelay())
		assert.Equal(t, 2.0, testutil.ToFloat64(p.errors.WithLabelValues(StageLogin)))
	})

	t.Run("resets the failure count on recovery", func(t *testing.T) {
		p := newTestPoller(&scriptedFetcher{results: []fetchResult{{err: errors.New("boom")}, {status: good}}})
		p.Poll(ctx)
		p.Poll(ctx)

		snap := p.Snapshot()
		assert.True(t, snap.Up)
		assert.Zero(t, snap.ConsecutiveFailures)
		assert.Equal(t, 30*time.Second, p.nextDelay())
		assert.Equal(t, 1.0, testutil.ToFloat64(p.errors.WithLabelValues(StageFetch)),
			"errors without a stage count as fetch errors")
	})

	t.Run("counts skipped records", func(t *testing.T) {
		p := newTestPoller(&scriptedFetcher{results: []fetchResult{{status: Status{SkippedDownstream: 2, SkippedUpstream: 1}}}})
		p.Poll(ctx)

		assert.Equal(t, 2.0, testutil.ToFloat64(p.skipped.WithLabelValues("downstream")))
		assert.Equal(t, 1.0, testutil.ToFloat64(p.skipped.WithLabelValues("upstream")))
	})

	t.Run("gives up on a poll after the timeout", func(t *testing.T) {
		p := NewPoller(blockingFetcher{}, PollerConfig{
			Interval: time.Second, Timeout: 20 * time.Millisecond, MaxBackoff: time.Second,
		}, prometheus.NewRegistry())

		done := make(chan struct{})
		go func() {
			p.Poll(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("poll did not honor its timeout")
		}
		assert.Equal(t, 1, p.Snapshot().ConsecutiveFailures)
	})
}

func TestPoller_Run(t *testing.T) {
	t.Run("stops when the context is cancelled", func(t *testing.T) {
		fetcher := &scriptedFetcher{results: []fetchResult{{status: Status{}}}}
		p := newTestPoller(fetcher)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			p.Run(ctx)
			close(done)
		}()
		require.Eventually(t, func() bool { return p.Snapshot().Up }, time.Second, time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Run did not return after cancel")
		}
	})
}
