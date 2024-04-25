package memory

// GetMemoryLimit returns system memory limit
// if cgroup is used, it returns cgroup memory limit
func GetMemoryLimit() uint64 {
	return sysTotalMemory()
}

// GetMemoryFree returns memory free
func GetMemoryFree() uint64 {
	return sysFreeMemory()
}
