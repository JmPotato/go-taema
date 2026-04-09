package taema

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZeroState(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(time.Second)

	re.False(ema.IsInitialized())
	re.Zero(ema.Value())
	re.True(ema.LastSampleTime().IsZero())
}

func TestFirstSampleRecordsTimeOnly(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(time.Second)
	now := time.Now()

	ema.Sample(now, 100)
	re.False(ema.IsInitialized())
	re.Zero(ema.Value())
	re.Equal(now, ema.LastSampleTime())
}

func TestBasicEMA(t *testing.T) {
	const delta = 0.1
	re := require.New(t)
	ema := NewEMA(time.Second)
	now := time.Now()

	// First sample: only records the timestamp.
	ema.Sample(now, 100)
	re.Zero(ema.Value())

	// Second sample: initializes EMA with rate = 100 / 1s = 100.
	now = now.Add(time.Second)
	ema.Sample(now, 100)
	re.True(ema.IsInitialized())
	re.Equal(100.0, ema.Value())

	// Third sample with the same rate: EMA stays near 100.
	now = now.Add(time.Second)
	ema.Sample(now, 100)
	re.InDelta(100.0, ema.Value(), delta)

	// Fourth sample with doubled value: rate = 200/1s, EMA moves toward 200.
	now = now.Add(time.Second)
	ema.Sample(now, 200)
	re.InDelta(150.0, ema.Value(), delta)
}

func TestConvergence(t *testing.T) {
	const (
		delta         = 0.1
		targetRate    = 10000.0
		maxIterations = 1000
	)
	re := require.New(t)
	ema := NewEMA(time.Second)
	now := time.Now()

	// Initialize with a different rate.
	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 100)

	// Feed the target rate repeatedly; EMA should converge.
	converged := false
	for i := 0; i < maxIterations; i++ {
		now = now.Add(time.Second)
		ema.Sample(now, targetRate)
		if math.Abs(ema.Value()-targetRate) < delta {
			converged = true
			break
		}
	}
	re.True(converged, "EMA should converge to the target rate")
}

func TestHalfLifeProperty(t *testing.T) {
	re := require.New(t)
	tau := 5 * time.Second
	ema := NewEMA(tau)
	now := time.Now()

	// Initialize EMA to 1000.
	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 1000)
	re.Equal(1000.0, ema.Value())

	// After exactly τ with zero input, EMA should decay to ~500 (half of 1000).
	// decay = e^{-ln(2)/τ · τ} = e^{-ln(2)} = 0.5
	// EMA = 0.5 * 1000 + 0.5 * 0 = 500
	now = now.Add(tau)
	ema.Sample(now, 0)
	re.InDelta(500.0, ema.Value(), 0.1)
}

func TestMinThreshold(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(time.Second, WithMinThreshold(1.0))
	now := time.Now()

	// Initialize EMA with a small rate.
	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 0.5)
	re.Equal(0.5, ema.Value())

	// After a large time gap with zero input, the EMA decays below threshold → clamped to 0.
	now = now.Add(10 * time.Second)
	ema.Sample(now, 0)
	re.Zero(ema.Value())
}

func TestMinThresholdDefault(t *testing.T) {
	re := require.New(t)
	// Without WithMinThreshold, small values should NOT be clamped.
	ema := NewEMA(time.Second)
	now := time.Now()

	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 0.001)
	re.Equal(0.001, ema.Value())

	now = now.Add(time.Second)
	ema.Sample(now, 0.001)
	re.Greater(ema.Value(), 0.0)
}

func TestNonPositiveDuration(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(time.Second)
	now := time.Now()

	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 100)
	re.Equal(100.0, ema.Value())

	// Same timestamp: no-op, state unchanged.
	ema.Sample(now, 999)
	re.Equal(100.0, ema.Value())
	re.Equal(now, ema.LastSampleTime())

	// Earlier timestamp: no-op, lastSampleTime must NOT move backward.
	ema.Sample(now.Add(-time.Second), 999)
	re.Equal(100.0, ema.Value())
	re.Equal(now, ema.LastSampleTime())
}

func TestNegativeValueClampedToZero(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(time.Second)
	now := time.Now()

	ema.Sample(now, 0)
	now = now.Add(time.Second)
	// Negative values are treated as 0 (rate = max(0, value) / dur).
	ema.Sample(now, -100)
	re.Zero(ema.Value())
}

func TestIrregularSampling(t *testing.T) {
	re := require.New(t)
	ema := NewEMA(5 * time.Second)
	now := time.Now()

	ema.Sample(now, 0)
	// Short interval: 100ms, value proportional to rate of 100/s.
	now = now.Add(100 * time.Millisecond)
	ema.Sample(now, 10) // rate = 10 / 0.1 = 100
	re.InDelta(100.0, ema.Value(), 0.1)

	// Long interval: 10s, same rate.
	now = now.Add(10 * time.Second)
	ema.Sample(now, 1000) // rate = 1000 / 10 = 100
	re.InDelta(100.0, ema.Value(), 0.1)
}

func TestConcurrentAccess(t *testing.T) {
	ema := NewEMA(time.Second)
	now := time.Now()

	// Initialize.
	ema.Sample(now, 0)
	now = now.Add(time.Second)
	ema.Sample(now, 100)

	var wg sync.WaitGroup
	// Concurrent writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ts := now.Add(time.Duration(offset*100+j) * time.Millisecond)
				ema.Sample(ts, 50)
			}
		}(i)
	}
	// Concurrent readers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = ema.Value()
				_ = ema.IsInitialized()
				_ = ema.LastSampleTime()
			}
		}()
	}
	wg.Wait()
}

func TestDifferentTimeConstants(t *testing.T) {
	re := require.New(t)
	now := time.Now()

	// A shorter time constant should react faster to changes.
	fast := NewEMA(1 * time.Second)
	slow := NewEMA(10 * time.Second)

	// Initialize both to rate 100.
	fast.Sample(now, 0)
	slow.Sample(now, 0)
	now = now.Add(time.Second)
	fast.Sample(now, 100)
	slow.Sample(now, 100)

	// Suddenly change rate to 1000.
	now = now.Add(time.Second)
	fast.Sample(now, 1000)
	slow.Sample(now, 1000)

	// The fast EMA should be closer to 1000 than the slow one.
	re.Greater(fast.Value(), slow.Value())
}
