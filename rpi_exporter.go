package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	// namespace for all metrics of this exporter.
	namespace = "rpi"
)

var (
	webListenAddress = flag.String("web.listen-address", ":9243", "Address on which to expose metrics and web interface.")
	webMetricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	showVersion      = flag.Bool("version", false, "Print version information.")
)

// landingPage contains the HTML served at '/'.
var landingPage = `<html>
	<head>
		<title>Raspberry Pi Exporter</title>
	</head>
	<body>
		<h1>Raspberry Pi Exporter</h1>
		<p>
		<a href=` + *webMetricsPath + `>Metrics</a>
		</p>
	</body>
</html>`

// The Exporter collects stats from the Raspberry Pi and exports them using the
// prometheus client library.
type Exporter struct {
	mutex sync.RWMutex

	// Basic exporter metrics.
	up, scrapeDuration          prometheus.Gauge
	totalScrapes, failedScrapes prometheus.Counter

	// Raspberry Pi metrics.
	temp prometheus.Gauge
}

// New returns a new, initialized Raspberry Pi Exporter.
func New() *Exporter {
	return &Exporter{
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of RPi metrics successful?",
		}),
		scrapeDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_duration_seconds",
			Help:      "Duration of the scrape of metrics from the RPi.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total RPi scrapes.",
		}),
		failedScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_failures_total",
			Help:      "Total amount of scrape failures.",
		}),

		temp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "cpu",
			Name:      "temperature_celsius",
			Help:      "CPU temperature in degrees celsius.",
		}),
	}
}

// Describe all the metrics collected by the Raspberry Pi exporter.
// Implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.up.Describe(ch)
	e.scrapeDuration.Describe(ch)
	e.failedScrapes.Describe(ch)
	e.totalScrapes.Describe(ch)
	e.temp.Describe(ch)
}

// Collect the stats and deliver them as Prometheus metrics.
// Implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	// Protect metrics from concurrent collects.
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Initial scrape.
	if err := e.scrape(); err != nil {
		log.Errorln("Scrape error:", err)
	}

	// Collect metrics.
	e.up.Collect(ch)
	e.scrapeDuration.Collect(ch)
	e.failedScrapes.Collect(ch)
	e.totalScrapes.Collect(ch)
	e.temp.Collect(ch)
}

// scrape is where the magic happens. FIXME: Better description.
func (e *Exporter) scrape() (err error) {
	// Evaluate if the scrape was successful or not.
	defer func() {
		if err == nil {
			e.up.Set(1)
		} else {
			e.up.Set(0)
		}
	}()

	// Meassure scrape duration.
	defer func(begun time.Time) {
		e.scrapeDuration.Set(time.Since(begun).Seconds())
	}(time.Now())

	e.totalScrapes.Inc()

	// Get temperature string from /sys/class/thermal/thermal_zone*/temp and
	// convert it to 64bit float value.
	b, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return err
	}
	temp, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	e.temp.Set(temp / 1000)

	return nil
}

func main() {
	flag.Parse()

	// If the version Flag is set, print detailed version information and exit.
	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("rpi_exporter"))
		os.Exit(0)
	}

	// Print build context and version.
	log.Infoln("Starting rpi_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	// Create a new RaspberryPi exporter.
	exporter := New()

	// Register RaspberryPi exporter and the collector for version information.
	// Unregister Go and Process collector which are registered by default.
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("rpi_exporter"))
	prometheus.Unregister(prometheus.NewGoCollector())
	prometheus.Unregister(prometheus.NewProcessCollector(os.Getpid(), ""))

	// Setup router and handlers.
	router := http.NewServeMux()
	metricsHandler := promhttp.HandlerFor(prometheus.DefaultGatherer,
		promhttp.HandlerOpts{ErrorLog: log.NewErrorLogger()})
	// TODO: InstrumentHandler is depracted. Additional tools will be available
	// soon in the promhttp package.
	//router.Handle(*webMetricsPath, prometheus.InstrumentHandler("prometheus", metricsHandler))
	router.Handle(*webMetricsPath, metricsHandler)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(landingPage))
	})

	// Setup webserver.
	srv := &http.Server{
		Addr:           *webListenAddress,
		Handler:        router,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
		ErrorLog:       log.NewErrorLogger(),
	}

	// Listen for termination signals.
	term := make(chan os.Signal)
	webErr := make(chan error)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	// Run webserver in a separate go-routine.
	log.Infoln("Listening on", *webListenAddress)
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

	// Did the context throw an error?
	if err := ctx.Err(); err != nil {
		log.Warnln(err)
	}

	log.Infoln("See you next time!")
}
