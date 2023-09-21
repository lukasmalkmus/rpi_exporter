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

package collector

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const cpuSubsystem = "cpu"

type cpuCollector struct {
	cpuTempCelsius *prometheus.Desc
	cpuFreqHertz   *prometheus.Desc
}

func init() {
	registerCollector("cpu", defaultEnabled, NewCPUCollector)
}

// NewCPUCollector returns a new Collector exposing CPU temperature metrics.
func NewCPUCollector() (Collector, error) {
	cc := &cpuCollector{
		cpuTempCelsius: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuSubsystem, "temperature_celsius"),
			"CPU temperature in degrees celsius (°C).",
			nil, nil,
		),
		cpuFreqHertz: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, cpuSubsystem, "frequency_hertz"),
			"CPU Frequency in hertz (Hz).",
			[]string{"cpu"}, nil,
		),
	}
	return cc, nil
}

// Update implements the Collector interface.
func (c *cpuCollector) Update(ch chan<- prometheus.Metric) error {
	// Get temperature string from /sys/class/thermal/thermal_zone0/temp and
	// convert it to float64 value.
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
		c.cpuTempCelsius,
		prometheus.GaugeValue, temp,
	)

	// Get all the cpus from /sys/devices/system/cpu/cpu*.
	cpus, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*")
	if err != nil {
		return err
	}

	for i, cpu := range cpus {
		// Get the frequency string from /sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq
		// and convert it to a float64 value.
		// Use scaling_cur_freq rather than cpuinfo_cur_freq because that seems to be
		// more accurate, according to the internet.
		b, err = ioutil.ReadFile(cpu + "/cpufreq/scaling_cur_freq")
		if err != nil {
			return err
		}
		freq, err := strconv.ParseFloat(string(bytes.TrimSpace(b)), 64)
		if err != nil {
			return err
		}

		// Export the metric.
		ch <- prometheus.MustNewConstMetric(
			c.cpuFreqHertz,
			prometheus.GaugeValue,
			freq,
			strconv.Itoa(i),
		)
	}

	return nil
}
