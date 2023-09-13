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
	"os/exec"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const gpuSubsystem = "gpu"

// The vcgencmd components to be considered part of the gpu.
// The clock frequency of these will be exported.
func getGpuComponents() []string {
	return []string{"core", "h264", "v3d"}
}

var (
	// /opt/vc/bin/vcgencmd for RaspiOS 32bit
	// /usr/bin/vcgencmd for RaspiOS 64bit
	vcgencmd = kingpin.Flag("vcgencmd", "vcgencmd including path.").Default("/opt/vc/bin/vcgencmd").String()
)

type gpuCollector struct {
	vcgencmd	string
	gpuTempCelsius	*prometheus.Desc
	gpuFreqHertz	*prometheus.Desc
}

func init() {
	registerCollector("gpu", defaultEnabled, NewGPUCollector)
}

// NewGPUCollector returns a new Collector exposing GPU temperature metrics.
func NewGPUCollector() (Collector, error) {
	gc := &gpuCollector{
		vcgencmd: *vcgencmd,
		gpuTempCelsius: prometheus.NewDesc(
                        prometheus.BuildFQName(namespace, gpuSubsystem, "temperature_celsius"),
                        "GPU temperature in degrees celsius (°C).",
                        nil, nil,
                ),
		gpuFreqHertz: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, gpuSubsystem, "frequency_hertz"),
			"GPU frequency in hertz (Hz).",
			[]string{"component"}, nil,
		),
	}
	return gc, nil
}

// Update implements the Collector interface.
func (c *gpuCollector) Update(ch chan<- prometheus.Metric) error {
	// Get temperature string by executing /opt/vc/bin/vcgencmd measure_temp
	// and convert it to float64 value.
	cmd := exec.Command(c.vcgencmd, "measure_temp")
	stdout, err := cmd.Output()
	if err != nil {
		return err
	}

	// temp=55.3'C => 55.3
	tempStr := string(stdout)
	idx := strings.IndexByte(tempStr, '=')
	if idx != -1 {
		tempStr = tempStr[idx + 1:]
	}
	tempStr = strings.TrimSuffix(tempStr, "'C\n")
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return err
	}

	// Export the metric.
	ch <- prometheus.MustNewConstMetric(
		c.gpuTempCelsius,
		prometheus.GaugeValue, temp,
	)

	for _, component := range getGpuComponents() {
		// Get frequency string by executing vcgencmd and
		// convert it to float64 value.
		cmd = exec.Command(c.vcgencmd, "measure_clock", component)
		stdout, err := cmd.Output()
		if err != nil {
			return err
		}

		// frequency(1)=400000000 => 400000000
		freqStr := string(stdout)
		idx = strings.IndexByte(freqStr, '=')
		if idx != -1 {
			freqStr = freqStr[idx + 1:]
		}
		freqStr = strings.TrimSuffix(freqStr, "\n")
		freq, err := strconv.ParseFloat(freqStr, 64)
		if err != nil {
			return err
		}

		// Export the metric.
		ch <- prometheus.MustNewConstMetric(
			c.gpuFreqHertz,
			prometheus.GaugeValue,
			freq,
			component,
		)
	}

	return nil
}
