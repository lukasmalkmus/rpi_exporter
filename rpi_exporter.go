// Copyright 2019 Lukas Malkmus
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/lukasmalkmus/rpi_exporter/collector"
)

// A wrapper around http.Handler to handle filtering.
// Caches already used filter combinations.
// Create a new handler using newHandler().
type handler struct {
	unfilteredHandler http.Handler
	// There are only three collectors in this program, so that's seven combinations at most.
	filteredHandlers        map[string]http.Handler
	exporterMetricsRegistry *prometheus.Registry
	includeExporterMetrics  bool
}

func newHandler(includeExporterMetrics bool) *handler {
	h := &handler{
		filteredHandlers:       make(map[string]http.Handler),
		includeExporterMetrics: includeExporterMetrics,
	}

	// Add default collectors, if they aren't disabled.
	if h.includeExporterMetrics {
		h.exporterMetricsRegistry = prometheus.NewRegistry()
		h.exporterMetricsRegistry.MustRegister(
			promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}),
			promcollectors.NewGoCollector(),
		)
	}

	// Create the unfiltered default handler.
	unfilteredHandler, err := h.filteredHandler()
	if err != nil {
		panic(fmt.Sprintf("Couldn't create metrics handler: %s", err))
	}

	h.unfilteredHandler = unfilteredHandler
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get the filters from the query.
	filters := r.URL.Query()["collect[]"]
	// Sort filters to allow caching of filtered handlers.
	sort.Strings(filters)
	log.Debugln("collect query:", filters)

	// Use the unfiltered handler if no filters were given.
	if len(filters) == 0 {
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}

	// Create a filtered handler.
	filteredHandler, err := h.filteredHandler(filters...)
	if err != nil {
		log.Errorln("Couldn't create filtered handler:", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Couldn't create filtered metrics handler: %s", err)))
		return
	}

	filteredHandler.ServeHTTP(w, r)
}

func (h *handler) filteredHandler(filters ...string) (http.Handler, error) {
	// Do not recreate unfiltered handler if it already exists.
	if len(filters) == 0 && h.unfilteredHandler != nil {
		return h.unfilteredHandler, nil
	}

	// Check if there is a handler for this combination of filters already.
	filtersStr := strings.Join(filters, ",")
	handler := h.filteredHandlers[filtersStr]
	if handler != nil {
		return handler, nil
	}

	// Create a new Raspberry Pi collector.
	rpiColl, err := collector.New(filters...)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create %s", err)
	}

	// Create a new prometheus registry and register the Raspberry Pi collector
	// on it.
	reg := prometheus.NewRegistry()
	if err := reg.Register(rpiColl); err != nil {
		return nil, fmt.Errorf("Couldn't register collector: %s", err)
	}
	reg.MustRegister(version.NewCollector("rpi_exporter"))

	// Delegate http serving to Prometheus client library, which will call
	// collector.Collect.
	if h.includeExporterMetrics {
		handler = promhttp.HandlerFor(
			prometheus.Gatherers{h.exporterMetricsRegistry, reg},
			promhttp.HandlerOpts{
				ErrorLog:      log.NewErrorLogger(),
				ErrorHandling: promhttp.HTTPErrorOnError,
				Registry:      h.exporterMetricsRegistry,
			})
		handler = promhttp.InstrumentMetricHandler(
			h.exporterMetricsRegistry, handler,
		)
	} else {
		handler = promhttp.HandlerFor(reg,
			promhttp.HandlerOpts{
				ErrorLog:      log.NewErrorLogger(),
				ErrorHandling: promhttp.HTTPErrorOnError,
			})
	}

	// Store handler in cache if it isn't unfiltered.
	if len(filters) > 0 {
		h.filteredHandlers[filtersStr] = handler
	}
	return handler, nil
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// A very simple health check.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"alive": true}`)
}

func main() {
	// Command line flags.
	var (
		webListenAddress          = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9243").String()
		webMetricsPath            = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		webHealthPath             = kingpin.Flag("web.healthcheck-path", "Path under which the exporter exposes its status.").Default("/health").String()
		webDisableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).").Bool()
	)

	// Setup the command line flags and commands.
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("rpi_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// Print build context and version.
	log.Info("Starting rpi_exporter", version.Info())
	log.Info("Build context", version.BuildContext())

	// Setup router and handlers.
	mux := http.NewServeMux()
	mux.Handle(*webMetricsPath, newHandler(!*webDisableExporterMetrics))
	mux.HandleFunc(*webHealthPath, HealthCheckHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Raspberry Pi Exporter</title></head>
			<body>
			<h1>Raspberry Pi Exporter</h1>
			<p><a href="` + *webMetricsPath + `">Metrics</a></p>
			<p><a href="` + *webHealthPath + `">Exporter health</a></p>
			</body>
			</html>`))
	})

	// Setup webserver.
	srv := &http.Server{
		Addr:         *webListenAddress,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     log.NewErrorLogger(),
	}

	// Listen for termination signals.
	term := make(chan os.Signal, 1)
	defer close(term)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(term)

	// Run webserver in a separate go-routine.
	log.Info("Listening on ", *webListenAddress)
	webErr := make(chan error)
	defer close(webErr)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			webErr <- err
		}
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
		if err := srv.Shutdown(ctx); err != nil {
			log.Error(err)
		}
	case err := <-webErr:
		log.Error("Error starting web server, exiting gracefully:", err)
	}
	log.Info("See you next time!")
}
