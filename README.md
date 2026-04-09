# go-taema

A Go library implementing the **Time-Aware Exponential Moving Average (EMA)** algorithm for computing smoothed rates from irregularly sampled cumulative data.

## Motivation

Conventional EMA algorithms assume data arrives at a fixed frequency, making them unsuitable for scenarios where the sampling interval varies. For example, in distributed systems, metrics reporting intervals often depend on workload intensity — high-throughput periods produce frequent samples while idle periods produce sparse ones.

This library introduces a time dimension to the EMA calculation. By using the decay factor `e^{-β·Δt}`, the algorithm naturally adapts to irregular sampling intervals: the larger the time gap between samples, the more the old data is "forgotten", and vice versa.

## Algorithm

The core formula:

```
β = ln(2) / τ
decay = e^{-β·Δt}
EMA_new = decay · EMA_old + (1 - decay) · rate
```

Where:

- **τ (tau)** is the time constant, also known as the **half-life** of the EMA. After exactly τ time has elapsed, the decay factor equals 0.5, meaning old data's weight is halved.
- **Δt** is the elapsed time since the last sample.
- **rate** is the instantaneous rate derived from the cumulative sample: `value / Δt`.

The relation between the decay factor and the time delta, described by `e^{-β·Δt}`:

![Decay factor illustration](https://github.com/user-attachments/assets/74d81499-fdec-49c8-b233-cf13782a065a)

## Installation

```bash
go get github.com/JmPotato/go-taema
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/JmPotato/go-taema"
)

func main() {
	// Create an EMA with a 5-second half-life.
	ema := taema.NewEMA(5*time.Second, taema.WithMinThreshold(1.0))

	now := time.Now()
	// First sample: records the baseline timestamp, no EMA computed yet.
	ema.Sample(now, 0)
	// Second sample: initializes the EMA with rate = 500 / 1s = 500.
	now = now.Add(time.Second)
	ema.Sample(now, 500)
	fmt.Println(ema.Value()) // 500

	// Subsequent samples update the EMA with time-aware decay.
	now = now.Add(time.Second)
	ema.Sample(now, 600)
	fmt.Println(ema.Value()) // ~513 (smoothed toward 600)
}
```

### API

| Method | Description |
|--------|-------------|
| `NewEMA(timeConstant, ...Option)` | Create a new EMA with the given half-life |
| `Sample(now, value)` | Feed a cumulative value at the given timestamp |
| `Value()` | Get the current smoothed rate |
| `IsInitialized()` | Whether at least one rate has been computed |
| `LastSampleTime()` | Timestamp of the most recent accepted sample |

### Options

- **`WithMinThreshold(v float64)`** — if the computed EMA falls below this threshold, it is clamped to 0 to avoid converging into a very small but non-zero value. Default is 0 (no clamping).

### Choosing a Time Constant

The time constant τ controls the trade-off between responsiveness and smoothness:

| τ | Behavior |
|---|----------|
| ~1-5s | Reacts quickly to spikes, but sensitive to short-term fluctuations |
| ~5-10s | Balanced smoothing with reasonable tracking speed |
| ~10-20s | Smooth output, suitable for observing long-term trends |

## References

- [Exponential Moving Averages for Irregular Time Series](https://oroboro.com/irregular-ema/) — Rafael Baptista
- [Exponential Weighted Averages with Irregular Sampling](http://tdunning.blogspot.com/2011/03/exponential-weighted-averages-with.html) — Ted Dunning
- [Operators on Inhomogeneous Time Series (PDF)](http://www.thalesians.com/archive/public/academic/finance/papers/Zumbach_2000.pdf) — Gilles Zumbach, 2000
- [Exponential Smoothing for Irregular Time Series (PDF)](https://www.kybernetika.cz/content/2008/3/385/paper.pdf) — Kybernetika, 2008

## License

[MIT](LICENSE)
