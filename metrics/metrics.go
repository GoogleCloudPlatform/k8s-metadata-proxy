package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "request_count",
			Help: "Number of metadata proxy requests broken down by filter result of request and HTTP response code.",
		},
		[]string{"filter_result", "code"},
	)
)

func init() {
	prometheus.MustRegister(RequestCounter)
}
