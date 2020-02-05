package main

import (
	"log"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/knl/pulley/internal/config"
	"github.com/knl/pulley/internal/metrics"
	"github.com/knl/pulley/internal/service"
	"github.com/knl/pulley/internal/version"
)

func main() {
	log.Println("server started")
	log.Println(version.Print())

	config, err := config.Setup()
	if err != nil {
		log.Fatal("Configuration step failed", err)
	}

	pulley := service.Pulley{
		Updates: make(chan interface{}, 100),
		Metrics: metrics.NewGithubMetrics(),
		Token:   config.WebhookToken,
	}

	pulley.MetricsProcessor(config.DefaultContextChecker(), config.TrackBuildTimes)

	// instrument the hook handler, so we could track how well we respond
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_in_flight_requests",
		Help: "A gauge of requests currently being served by the webhook handler.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_total",
			Help: "A counter for requests to the webhook handler.",
		},
		[]string{"code", "method"},
	)

	// duration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request duration.
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.05, .1, .25, 1, 2.5, 10},
		},
		[]string{"method"},
	)

	// requestSize has no labels, making it a zero-dimensional
	// ObserverVec.
	requestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_request_size_bytes",
			Help:    "A histogram of request sizes.",
			Buckets: []float64{1024, 2048, 4096, 16 * 1024, 64 * 1024},
		},
		[]string{},
	)

	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(inFlightGauge, counter, duration, requestSize)

	// Instrument the handlers with all the metrics, injecting the "handler"
	// label by currying.
	chain := promhttp.InstrumentHandlerInFlight(inFlightGauge,
		promhttp.InstrumentHandlerDuration(duration,
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerRequestSize(requestSize, pulley.HookHandler()),
			),
		),
	)

	http.Handle("/"+config.WebhookPath, chain)
	http.Handle("/"+config.MetricsPath, promhttp.Handler())

	// Listen & Serve
	addr := net.JoinHostPort(config.Host, config.Port)
	log.Printf("[service] listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
