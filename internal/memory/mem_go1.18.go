//go:build go1.18 && !go1.19 && goexperiment.pacerredesign
// +build go1.18,!go1.19,goexperiment.pacerredesign

package memory

import (
	"runtime"
)

// GetLiveDatasetSize returns the live dataset size in bytes which required by calculating GOGC
//
// In Go1.18, when the pacerredesign feature is enabled by default,
//
//	gcPercentHeapGoal = c.heapMarked + (c.heapMarked+c.lastStackScan.Load()+c.globalsScan.Load())*uint64(gcPercent)/100
//
// use `/memory/classes/heap/objects:bytes` metric as the estimate of live heap size(heapMarked), and
// use `/memory/classes/heap/stacks:bytes` metric as the estimate of live stack size(lastStackScan),
// both the return value will be overestimated.
func GetLiveDatasetSize() uint64 {
	liveHeapSize, err := readMetric("/memory/classes/heap/objects:bytes")
	stackSize, err2 := readMetric("/memory/classes/heap/stacks:bytes")
	if err == nil && err2 == nil {
		return liveHeapSize + stackSize
	}
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats) // this call will stop the world
	return stats.HeapAlloc + stats.StackInuse
}
