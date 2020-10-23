package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

type Metrics struct {
	RPTSummaryMs   prometheus.Summary
	RPTHistogramMs prometheus.Histogram
}

func New() *Metrics {

	m := &Metrics{
		RPTSummaryMs: prometheus.NewSummary(
			prometheus.SummaryOpts{
				Name:       "me_request_processing_time_summary_ms",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			}),
		RPTHistogramMs: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "me_request_processing_time_histogram_ms",
				Buckets: prometheus.LinearBuckets(0, 10, 20),
			}),
	}

	prometheus.MustRegister(m.RPTSummaryMs)
	prometheus.MustRegister(m.RPTHistogramMs)

	return m
}

func (m *Metrics) RunApi() error {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("EXTENDER_API_PORT")), nil)
	if err != nil {
		return err
	}
	return nil
}
