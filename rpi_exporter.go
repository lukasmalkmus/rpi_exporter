package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/lukasmalkmus/rpi_exporter/collector"
)

func handler(w http.ResponseWriter, r *http.Request) {
	// Get the filters from the query.
	filters := r.URL.Query()["collect[]"]
	log.Debugln("collect query:", filters)

	// Create a new Raspberry Pi collector.
	nc, err := collector.New(filters...)
	if err != nil {
		log.Warnln("Couldn't create", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Couldn't create %s", err)))
		return
	}

	// Create a new prometheus registry and register the Raspberry Pi collector
	// on it.
	reg := prometheus.NewRegistry()
	if err := reg.Register(nc); err != nil {
		log.Errorln("Couldn't register collector:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Couldn't register collector: %s", err)))
		return
	}

	// Delegate http serving to Prometheus client library, which will call
	// collector.Collect.
	h := promhttp.InstrumentMetricHandler(reg, promhttp.HandlerFor(reg,
		promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.HTTPErrorOnError,
		}),
	)
	h.ServeHTTP(w, r)
}

func main() {
	// Command line flags.
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9243").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)

	// Setup the command line flags and commands.
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("rpi_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// Print build context and version.
	log.Infoln("Starting rpi_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	// Setup router and handlers.
	r := http.NewServeMux()
	r.HandleFunc(*metricsPath, handler)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Raspberry Pi Exporter</title></head>
			<body>
			<h1>Raspberry Pi Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	// Setup webserver.
	srv := &http.Server{
		Addr:         *listenAddress,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     log.NewErrorLogger(),
	}

	// Listen for termination signals.
	term := make(chan os.Signal)
	defer close(term)
	webErr := make(chan error)
	defer close(webErr)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(term)

	// Run webserver in a separate go-routine.
	log.Infoln("Listening on", *listenAddress)
	go func() {
		webErr <- srv.ListenAndServe()
	}()

	// Wait for a termination signal and shut down gracefully, but wait no
	// longer than 5 seconds before halting.
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	select {
	case <-term:
		log.Warn("Received SIGTERM, exiting gracefully...")
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	case err := <-webErr:
		log.Errorln("Error starting web server, exiting gracefully:", err)
	}
	log.Infoln("See you next time!")
}
