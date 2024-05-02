package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Exporter interface {
	ReportSuccess(power, rCurrent, tCurrent float64)
	ReportFailure()
	http.Handler
}

type exporter struct {
	registry                             *prometheus.Registry
	powerConsumption, rCurrent, tCurrent prometheus.Gauge
	total                                prometheus.Counter
	successful                           prometheus.Counter
	handler                              http.Handler
}

func NewExporter() Exporter {
	registry := prometheus.NewRegistry()

	powerConsumption := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "power",
		Help:      "A gauge of power consumption in watts.",
		Namespace: "hems",
	})

	rCurrent := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "r_current",
		Help:      "A gauge of r current in ampere.",
		Namespace: "hems",
	})

	tCurrent := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "t_current",
		Help:      "A gauge of t current in ampere.",
		Namespace: "hems",
	})

	total := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "total",
		Help:      "A total counter of fetching power consumption.",
		Namespace: "hems",
	})

	successful := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "successful",
		Help:      "A counter of successful fetching power consumption.",
		Namespace: "hems",
	})

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		powerConsumption,
		rCurrent,
		tCurrent,
		total,
		successful,
	)

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	return &exporter{
		registry:         registry,
		powerConsumption: powerConsumption,
		rCurrent:         rCurrent,
		tCurrent:         tCurrent,
		total:            total,
		successful:       successful,
		handler:          handler,
	}
}

func (e *exporter) ReportSuccess(power, rCurrent, tCurrent float64) {
	e.powerConsumption.Set(power)
	e.rCurrent.Set(rCurrent)
	e.tCurrent.Set(tCurrent)

	e.total.Inc()
	e.successful.Inc()
}

func (e *exporter) ReportFailure() {
	e.total.Inc()
}

func (e *exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.handler.ServeHTTP(w, r)
}
