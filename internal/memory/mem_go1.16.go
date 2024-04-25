//go:build go1.16 && !go1.18
// +build go1.16,!go1.18

package memory

import (
	"runtime"
)

// GetLiveDatasetSize returns the live dataset size in bytes which required by calculating GOGC
// 	gc goal = memstats.heap_marked + memstats.heap_marked*uint64(gcpercent)/100
// use `/memory/classes/heap/objects:bytes` metric as the estimate of `heap_marked`, the return value will be overestimated.
func GetLiveDatasetSize() uint64 {
	liveHeapSize, err := readMetric("/memory/classes/heap/objects:bytes")
	if err == nil {
		return liveHeapSize
	}
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats) // this call will stop the world
	return stats.HeapAlloc
}
