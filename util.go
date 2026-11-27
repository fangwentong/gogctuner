package gogctuner

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/fangwentong/gogctuner/internal/memory"
)

func setGOGCOrDefault(gogc int, logger Logger, hooks runtimeHooks) {
	if gogc != 0 {
		logger.Logf("gctuner: SetGCPercent %v", gogc)
		hooks.setGCPercent(gogc)
		return
	}

	defaultGOGC := readGOGC(hooks.getEnv)
	logger.Logf("gctuner: SetGCPercent to default: %v", defaultGOGC)
	hooks.setGCPercent(defaultGOGC)
}

// readGOGC reads the GOGC value from the environment.
// Copied from runtime.readGOGC.
func readGOGC(getEnv func(string) string) int {
	p := getEnv("GOGC")
	if p == "off" {
		return -1
	}
	if n, err := strconv.Atoi(p); err == nil {
		return n
	}
	return 100
}

func getMemoryLimit() (uint64, error) {
	limit := memory.GetMemoryLimit()
	if limit == 0 {
		return 0, errors.New("gctuner: failed to get memory limit")
	}
	return limit, nil
}

// printMemorySize prints memory size in a readable format.
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
