//go:build !go1.16
// +build !go1.16

package memory

import "runtime"

// GetLiveDatasetSize returns the live dataset size in bytes which required by calculating GOGC
// 	gc goal = memstats.heap_marked + memstats.heap_marked*uint64(gcpercent)/100
// use `HeapAlloc` in MemStats as the estimate of `heap_marked`, the return value will be overestimated.
func GetLiveDatasetSize() uint64 {
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats) // this call will stop the world
	return stats.HeapAlloc
}
