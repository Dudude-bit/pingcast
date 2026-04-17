package domain

import "time"

// ChartPoint is an aggregated data point for the monitor response-time chart.
type ChartPoint struct {
	Timestamp     time.Time
	AvgResponseMs float64
	CheckCount    int
}
