package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

type Metrics struct {
	BlockHistogramMs prometheus.SummaryVec
}

func New() *Metrics {

	m := &Metrics{
		//BlockHistogramMs: prometheus.NewSummaryVec(
		//	prometheus.SummaryOpts{
		//		Name: "me_block_processing_ms",
		//	},
		//	[]string{"test"}),
	}

	//prometheus.MustRegister(m.BlockHistogramMs)

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
