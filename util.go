package gogctuner

import (
	"errors"
	"fmt"
	"github.com/fangwentong/gogctuner/internal/memory"
	"math"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
)

const (
	maxRAMUsagePercentage = 95
	minGOGCValue          = 50
	minHeapSize           = 4 << 20 // 4MB
	goGCNoLimit           = float64(math.MaxInt64)
)

// setGCParameter sets GC parameters
func adjustGOGCByMemoryLimit(oldConfig, newConfig Config, logger Logger) {
	if newConfig.MaxRAMPercentage > 0 {
		// If MaxRAMPercentage is set, adjust GOGC based on the current heap size and the target memory limit
		maxGOGC := goGCNoLimit
		if newConfig.GOGC > 0 {
			maxGOGC = float64(newConfig.GOGC)
		}
		getCurrentPercentAndChangeGOGC(newConfig.MaxRAMPercentage, maxGOGC, logger)
		return
	}
	if reflect.DeepEqual(oldConfig, newConfig) {
		return
	}
	// If MaxRAMPercentage is not set, set a static GOGC
	setGOGCOrDefault(newConfig.GOGC, logger)
}

func setGOGCOrDefault(gogc int, logger Logger) {
	if gogc != 0 {
		logger.Logf("gctuner: SetGCPercent %v", gogc)
		debug.SetGCPercent(gogc)
		return
	}

	// Reset GOGC to its default value
	defaultGOGC := readGOGC()
	logger.Logf("gctuner: SetGCPercent to default: %v", defaultGOGC)
	debug.SetGCPercent(defaultGOGC)
}

// readGOGC reads the GOGC value
// Copied from runtime.readGOGC
func readGOGC() int {
	p := os.Getenv("GOGC")
	if p == "off" {
		return -1
	}
	if n, err := strconv.Atoi(p); err == nil {
		return n
	}
	return 100
}

func getCurrentPercentAndChangeGOGC(memoryLimitInPercent float64, maxGOGC float64, logger Logger) {
	totalMemSize, err := getMemoryLimit()
	if err != nil {
		logger.Errorf("gctuner: failed to adjust GC, get memory limit err: %v", err.Error())
		return
	}

	liveHeapSize := memory.GetLiveDatasetSize()

	liveSize := math.Max(minHeapSize, float64(liveHeapSize))
	newgogc := getGOGC(memoryLimitInPercent, totalMemSize, liveSize, maxGOGC)

	logger.Logf("gctuner: limit %.2f%% (%s). adjusting GOGC to %d, live+unmarked %s",
		memoryLimitInPercent, printMemorySize(uint64(memoryLimitInPercent/100*float64(totalMemSize))),
		newgogc, printMemorySize(liveHeapSize))
	debug.SetGCPercent(newgogc)
}

func getGOGC(memoryLimitInPercent float64, totalMemSize uint64, liveSize float64, maxGOGC float64) int {
	// hard_target = live_dataset + live_dataset * (GOGC / 100).
	// hard_target = memoryLimitInPercent
	// live_dataset = memPercent
	// Therefore, gogc = (hard_target - live_dataset) / live_dataset * 100
	newgogc := calculateGOGC(memoryLimitInPercent, totalMemSize, liveSize)

	if newgogc > 0 {
		return int(math.Min(math.Max(newgogc, minGOGCValue), maxGOGC))
	}

	// If the current memory usage has already exceeded the target threshold, it is impossible to reach the target threshold no matter how GOGC is set
	// When setting the GOGC value, it is necessary to consider constraints such as GC overhead and memory limits, and within these constraints, a smaller GOGC value should be set
	// Considering GC overhead, it is not advisable to set a too low GOGC value, otherwise, the GC overhead will be high. We set a relatively low GOGC value as a safety net (minGOGCValue = 50)
	// Considering memory limits, if the maximum allowed memory percentage is maxMemPercent, then the upper limit for GOGC is (maxMemPercent - currentMemPercent) / memPercent * 100.0
	// Without considering the use of swap memory, the upper limit for maxMemPercent is 100%. If the out-of-memory killer is enabled, maxMemPercent should be reduced appropriately, such as 95%
	defaultGOGC := readGOGC()
	maxGOGC = math.Min(maxGOGC, calculateGOGC(maxRAMUsagePercentage, totalMemSize, liveSize))

	return int(math.Max(minGOGCValue, math.Min(float64(defaultGOGC), maxGOGC)))
}

func calculateGOGC(memoryLimitInPercent float64, memTotal uint64, liveSize float64) float64 {
	target := memoryLimitInPercent * float64(memTotal) / 100
	return (target - liveSize) / liveSize * 100.0
}

func getMemoryLimit() (uint64, error) {
	limit := memory.GetMemoryLimit()
	if limit == 0 {
		return 0, errors.New("gctuner: failed to get memory limit")
	}
	return limit, nil
}

// printMemorySize prints memory size in a readable format
func printMemorySize(bytes uint64) string {
	if bytes == uint64(math.MaxInt64) {
		return "unlimited"
	}
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)

	var unit string
	var value float64

	switch {
	case bytes < KB:
		unit = "B"
		value = float64(bytes)
	case bytes < MB:
		unit = "KB"
		value = float64(bytes) / KB
	case bytes < GB:
		unit = "MB"
		value = float64(bytes) / MB
	default:
		unit = "GB"
		value = float64(bytes) / GB
	}

	return fmt.Sprintf("%.2f%s", value, unit)
}
