package memory

import (
	"fmt"
	"github.com/fangwentong/gogctuner/internal/cgroup"
	"github.com/pbnjay/memory"
)

func sysTotalMemory() uint64 {
	totalMem := memory.TotalMemory()
	if totalMem == 0 {
		panic(fmt.Sprintf("FATAL: cannot determine system memory"))
	}
	mem := cgroup.GetMemoryLimit()
	if mem <= 0 || int64(int(mem)) != mem || uint64(mem) > totalMem {
		// Try reading hierarchical memory limit.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/699
		mem = cgroup.GetHierarchicalMemoryLimit()
		if mem <= 0 || int64(int(mem)) != mem || uint64(mem) > totalMem {
			return totalMem
		}
	}
	return uint64(mem)
}

func sysFreeMemory() uint64 {
	total := sysTotalMemory()
	usage := cgroup.GetMemoryUsage()
	if usage <= 0 || int64(int(usage)) != usage || uint64(usage) > total {
		return memory.FreeMemory()
	}
	return total - uint64(usage)
}
