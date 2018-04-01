package collector

import (
	"bytes"
	"io/ioutil"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const cpuSubsystem = "cpu"

type cpuCollector struct {
	mutex sync.Mutex
}

func init() {
	registerCollector("cpu", defaultEnabled, NewCPUCollector)
}

// NewCPUCollector returns a new Collector exposing CPU temperature metrics.
func NewCPUCollector() (Collector, error) {
	return &cpuCollector{}, nil
}

// Update implements the Collector interface.
func (c *cpuCollector) Update(ch chan<- prometheus.Metric) error {
	// Get temperature string from /sys/class/thermal/thermal_zone*/temp and
	// convert it to 64bit float value.
	b, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return err
	}
	temp, err := strconv.ParseFloat(string(bytes.TrimSpace(b)), 64)
	if err != nil {
		return err
	}
	temp = temp / 1000

	// Export the metric.
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuSubsystem, "temperature_celsius"),
			"CPU temperature in degrees celsius",
			nil, nil,
		),
		prometheus.GaugeValue, temp,
	)
	return nil
}
