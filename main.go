package main

import (
	"log"
	"net"
	"net/http"

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

	metrics := metrics.NewGithubMetrics()
	updates := service.MetricsProcessor(config.DefaultContextChecker(), metrics)

	metrics.Setup()
	http.HandleFunc("/"+config.WebhookPath, service.HookHandler(config.WebhookToken, updates))
	http.Handle("/"+config.MetricsPath, promhttp.Handler())

	// Listen & Serve
	addr := net.JoinHostPort(config.Host, config.Port)
	log.Printf("[service] listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
