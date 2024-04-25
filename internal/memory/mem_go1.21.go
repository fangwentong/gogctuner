//go:build go1.21
// +build go1.21

package memory

import (
	"runtime"
)

// GetLiveDatasetSize returns the live dataset size in bytes which required by calculating GOGC
// since go1.19,
//	gcPercentHeapGoal = c.heapMarked + (c.heapMarked+c.lastStackScan.Load()+c.globalsScan.Load())*uint64(gcPercent)/100
//
// There are some metrics be added in go1.21 to accurately get `heapMarked` and `lastStackScan`,
// use `/gc/heap/live:bytes` metric as `heapMarked`, and use `/gc/scan/stack:bytes` metric as `lastStackScan`
func GetLiveDatasetSize() uint64 {
	liveHeapSize, err := readMetric("/gc/heap/live:bytes")
	stackSize, err2 := readMetric("/gc/scan/stack:bytes")
	if err == nil && err2 == nil {
		return liveHeapSize + stackSize
	}
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats) // this call will stop the world
	return stats.HeapAlloc + stats.StackInuse
}
