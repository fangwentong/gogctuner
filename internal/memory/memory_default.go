//go:build !linux
// +build !linux

package memory

import (
	"github.com/pbnjay/memory"
)

func sysTotalMemory() uint64 {
	return memory.TotalMemory()
}

func sysFreeMemory() uint64 {
	return memory.FreeMemory()
}
