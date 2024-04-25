//go:build go1.16
// +build go1.16

package memory

import (
	"errors"
	"fmt"
	"runtime/metrics"
)

func readMetric(metricName string) (uint64, error) {
	// Create a sample for the metric.
	sample := make([]metrics.Sample, 1)
	sample[0].Name = metricName

	// Sample the metric.
	metrics.Read(sample)

	// Check if the metric is actually supported.
	// If it's not, the resulting value will always have
	// kind KindBad.
	if sample[0].Value.Kind() == metrics.KindBad {
		return 0, errors.New(fmt.Sprintf("metric %q no longer supported", metricName))
	}

	// Handle the result.
	//
	// It's OK to assume a particular Kind for a metric;
	// they're guaranteed not to change.
	freeBytes := sample[0].Value.Uint64()
	return freeBytes, nil
}
