// Package taema provides a time-aware exponential moving average (EMA) algorithm
// designed for computing smoothed rates from irregularly sampled cumulative data.
//
// In many real-world systems, data arrives at non-fixed intervals. For example,
// a distributed system may report resource consumption every 1s under heavy load
// but every 10s when idle. Conventional EMA algorithms assume fixed-frequency
// sampling and produce distorted results when the interval varies.
//
// This package uses the formula e^{-β·Δt} as the decay factor, where β = ln(2) / τ
// and τ is the configurable time constant (half-life). The decay naturally adapts to
// irregular intervals: large gaps "forget" old data faster, small gaps preserve it.
//
// Each call to [EMA.Sample] provides a cumulative value consumed since the last
// sample. The library derives the instantaneous rate (value / Δt) and applies
// time-aware exponential smoothing to produce a stable rate estimate.
package taema

import (
	"math"
	"sync"
	"time"
)

// Option configures the [EMA] instance.
type Option func(*EMA)

// WithMinThreshold sets the minimum threshold for the EMA value.
// If the computed EMA falls below this threshold, it is clamped to 0
// to avoid converging into a very small but non-zero value.
func WithMinThreshold(v float64) Option {
	return func(t *EMA) {
		t.minThreshold = v
	}
}

// EMA is a time-aware exponential moving average calculator.
// It is safe for concurrent use by multiple goroutines.
type EMA struct {
	mu          sync.RWMutex
	initialized bool
	// beta = ln(2) / τ, where τ is the time constant (half-life) of the EMA.
	// When Δt equals τ, the decay factor e^{-β·Δt} equals 0.5, meaning the
	// weight of the old data is halved.
	beta           float64
	minThreshold   float64
	lastSampleTime time.Time
	lastEMA        float64
}

// NewEMA creates a new [EMA] instance with the given time constant (half-life).
//
// The time constant τ determines how quickly old data is "forgotten":
//   - A smaller τ (e.g., 5s) reacts quickly to changes but is more sensitive to fluctuations.
//   - A larger τ (e.g., 20s) smooths out fluctuations but tracks changes more slowly.
//
// The timeConstant must be positive; otherwise, the behavior is undefined.
func NewEMA(timeConstant time.Duration, opts ...Option) *EMA {
	t := &EMA{
		beta: math.Log(2) / timeConstant.Seconds(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Sample records a new data point at the given time.
//
// The value parameter represents the total quantity accumulated since the last
// sample. The rate is derived internally as value / Δt, where Δt is the elapsed
// time since the previous sample.
//
// On the first call, only the timestamp is recorded — no EMA is computed because
// there is no prior reference point to measure Δt. Starting from the second call,
// the EMA is updated using the time-aware decay factor.
//
// If now is not after the last sample time (i.e., Δt <= 0), the call is a no-op
// and the internal state is not modified.
func (t *EMA) Sample(now time.Time, value float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// First sample: record the baseline timestamp and return.
	if t.lastSampleTime.IsZero() {
		t.lastSampleTime = now
		return
	}

	// Reject out-of-order or duplicate timestamps.
	dur := now.Sub(t.lastSampleTime)
	if dur <= 0 {
		return
	}
	t.lastSampleTime = now

	// Derive the instantaneous rate from the cumulative value.
	rate := math.Max(0, value) / dur.Seconds()

	// Second sample: initialize the EMA directly with the first observed rate.
	if !t.initialized {
		t.initialized = true
		t.lastEMA = rate
		return
	}

	// Apply the time-aware decay factor.
	//   decay → 1 when Δt is small (old data is preserved)
	//   decay → 0 when Δt is large (old data is forgotten)
	decay := math.Exp(-t.beta * dur.Seconds())
	t.lastEMA = decay*t.lastEMA + (1-decay)*rate

	// Clamp to zero if below the minimum threshold.
	if t.minThreshold > 0 && t.lastEMA < t.minThreshold {
		t.lastEMA = 0
	}
}

// Value returns the current EMA value.
// Returns 0 if no valid samples have been recorded yet.
func (t *EMA) Value() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEMA
}

// IsInitialized reports whether the EMA has computed at least one valid rate.
// This requires at least two [EMA.Sample] calls with increasing timestamps.
func (t *EMA) IsInitialized() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.initialized
}

// LastSampleTime returns the timestamp of the most recent accepted sample.
// Returns the zero [time.Time] if no samples have been recorded.
func (t *EMA) LastSampleTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastSampleTime
}
