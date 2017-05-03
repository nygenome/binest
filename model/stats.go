package model

import "math"

// OnlineStats holds the current state for estimating
// single pass mean and variance from a large float64 stream
// For more information see Knuth (TAOCP Vol 2, 3rd ed, pg 232).
type OnlineStats struct {
	diff  float64
	Min   float64
	Max   float64
	Mean  float64
	Count uint64
}

// Clear resets the state of the OnlineStats instance
func (o *OnlineStats) Clear() {
	o.diff = 0.0
	o.Min = 0.0
	o.Max = 0.0
	o.Mean = 0.0
	o.Count = 0
}

// Variance returns the variance of the data stream
func (o *OnlineStats) Variance() float64 {
	if o.Count > 1 {
		return o.diff / float64(o.Count-1)
	}
	return 0.0
}

// SD returns the standard deviation of the data stream
func (o *OnlineStats) SD() float64 {
	return math.Sqrt(o.Variance())
}

// Push pushes value to the OnlineStats data stream
func (o *OnlineStats) Push(value float64) {
	if o.Count == 0 {
		o.Min = value
		o.Max = value
	} else {
		if value < o.Min {
			o.Min = value
		}
		if value > o.Max {
			o.Max = value
		}
	}

	o.Count++
	prev_mean := o.Mean
	o.Mean += (value - prev_mean) / float64(o.Count)
	o.diff += (value - prev_mean) * (value * o.Mean)
}

// NewOnlineStats returns a new instance of OnlineStats
func NewOnlineStats() *OnlineStats {
	return &OnlineStats{}
}
