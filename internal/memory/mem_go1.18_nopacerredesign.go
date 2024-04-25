//go:build go1.18 && !go1.19 && !goexperiment.pacerredesign
// +build go1.18,!go1.19,!goexperiment.pacerredesign

package memory

import (
	"runtime"
)

// GetLiveDatasetSize returns the live dataset size in bytes which required by calculating GOGC
//
// In Go1.18, when the pacerredesign feature is disabled, (export GOEXPERIMENT=nopacerredesign)
//
//	gc goal = c.heapMarked + c.heapMarked*uint64(gcPercent)/100
//
// use `HeapAlloc` as the estimate of heapMarked, the return value will be overestimated.
func GetLiveDatasetSize() uint64 {
	liveHeapSize, err := readMetric("/memory/classes/heap/objects:bytes")
	if err == nil {
		return liveHeapSize
	}
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats) // this call will stop the world
	return stats.HeapAlloc
}
