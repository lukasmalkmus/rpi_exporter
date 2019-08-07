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
)

const gpuSubsystem = "gpu"

type gpuCollector struct{}

func init() {
	registerCollector("gpu", defaultEnabled, NewGPUCollector)
}

// NewGPUCollector returns a new Collector exposing GPU temperature metrics.
func NewGPUCollector() (Collector, error) {
	return &gpuCollector{}, nil
}

// Update implements the Collector interface.
func (c *gpuCollector) Update(ch chan<- prometheus.Metric) error {
	// Get temperature string by executing /opt/vc/bin/vcgencmd measure_temp
	// and convert it to float64 value.
	cmd := exec.Command("/opt/vc/bin/vcgencmd", "measure_temp")
	stdout, err := cmd.Output()
	if err != nil {
		return err
	}

	// temp=55.3'C => 55.3
	tempStr := strings.TrimPrefix(string(stdout), "temp=")
	tempStr = strings.TrimSuffix(tempStr, "'C\n")
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return err
	}

	// Export the metric.
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			prometheus.BuildFQName(namespace, gpuSubsystem, "temperature_celsius"),
			"GPU temperature in degrees celsius (°C).",
			nil, nil,
		),
		prometheus.GaugeValue, temp,
	)
	return nil
}
