package model

import "time"

type Metric struct {
	Namespace  string
	Name       string
	Dimensions []MetricDimension
}

type MetricDimension struct {
	Name  string
	Value string
}

type MetricStatistic struct {
	Timestamp time.Time
	Value     float64
}

type MetricStatisticList []MetricStatistic

func (f MetricStatisticList) Len() int {
	return len(f)
}

func (f MetricStatisticList) Less(i, j int) bool {
	return f[i].Timestamp.Before(f[j].Timestamp)
}

func (f MetricStatisticList) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
