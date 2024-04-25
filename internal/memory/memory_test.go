package memory_test

import (
	"fmt"
	"github.com/fangwentong/gogctuner/internal/memory"
	"log"
	"testing"
)

func TestGetMemoryLimit(t *testing.T) {
	limit := memory.GetMemoryLimit()
	free := memory.GetMemoryFree()
	log.Printf("memory limit: %s, free: %s", printMemorySize(limit), printMemorySize(free))
}

// printMemorySize prints memory size in readable format
func printMemorySize(bytes uint64) string {
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
