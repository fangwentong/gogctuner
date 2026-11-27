package cgroup

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultProcCgroupPath    = "/proc/self/cgroup"
	defaultProcMountinfoPath = "/proc/self/mountinfo"
	defaultCgroupV2Root      = "/sys/fs/cgroup"
)

// GetMemoryLimit returns the effective cgroup memory limit (v2 ancestor-chain minimum, falling back to v1).
func GetMemoryLimit() int64 {
	if n, err := getMemoryLimitV2(defaultProcCgroupPath, defaultProcMountinfoPath, defaultCgroupV2Root); err == nil && n > 0 {
		return n
	}
	if n, err := getMemStat("memory.limit_in_bytes"); err == nil {
		return n
	}
	return 0
}

// GetMemoryUsage returns current cgroup memory usage (v2 first, falling back to v1).
func GetMemoryUsage() int64 {
	if n, err := getMemoryUsageV2(defaultProcCgroupPath, defaultProcMountinfoPath, defaultCgroupV2Root); err == nil && n > 0 {
		return n
	}
	if n, err := getMemStat("memory.usage_in_bytes"); err == nil {
		return n
	}
	return 0
}

func getMemStat(statName string) (int64, error) {
	return getStatGeneric(statName, "/sys/fs/cgroup/memory", defaultProcCgroupPath, "memory")
}

// GetHierarchicalMemoryLimit returns cgroup v1 hierarchical memory limit.
// https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
func GetHierarchicalMemoryLimit() int64 {
	n, err := getHierarchicalMemoryLimit("/sys/fs/cgroup/memory", defaultProcCgroupPath)
	if err != nil {
		return 0
	}
	return n
}

func getHierarchicalMemoryLimit(sysfsPrefix, cgroupPath string) (int64, error) {
	data, err := getFileContents("memory.stat", sysfsPrefix, cgroupPath, "memory")
	if err != nil {
		return 0, err
	}
	memStat, err := grepFirstMatch(data, "hierarchical_memory_limit", 1, " ")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(memStat, 10, 64)
}

type cgroupV2Mount struct {
	root       string
	mountPoint string
}

// resolveCgroupV2Dir resolves the effective cgroup v2 directory and mount root using
// /proc/self/cgroup (0::<path>) and /proc/self/mountinfo (handling non-"/" mount roots
// and octal-escaped paths).
func resolveCgroupV2Dir(procCgroupFile, procMountinfoFile, fallbackMountRoot string) (curDir, mountRoot string, err error) {
	relPath, err := parseCgroupV2Path(procCgroupFile)
	if err != nil {
		return "", "", err
	}
	relPath = "/" + strings.TrimPrefix(relPath, "/")

	mounts := parseCgroupV2Mounts(procMountinfoFile)
	var chosen *cgroupV2Mount
	for i := range mounts {
		m := &mounts[i]
		mRoot := "/" + strings.TrimPrefix(m.root, "/")
		if mRoot == "/" || relPath == mRoot || strings.HasPrefix(relPath, strings.TrimSuffix(mRoot, "/")+"/") {
			if chosen == nil || len(mRoot) > len(chosen.root) {
				chosen = &cgroupV2Mount{root: mRoot, mountPoint: m.mountPoint}
			}
		}
	}

	var subPath string
	if chosen != nil {
		mountRoot = filepath.Clean(chosen.mountPoint)
		if chosen.root == "/" {
			subPath = strings.TrimPrefix(relPath, "/")
		} else {
			subPath = strings.TrimPrefix(strings.TrimPrefix(relPath, chosen.root), "/")
		}
	} else {
		mountRoot = filepath.Clean(fallbackMountRoot)
		subPath = strings.TrimPrefix(relPath, "/")
	}

	curDir = filepath.Clean(filepath.Join(mountRoot, filepath.FromSlash(subPath)))
	if !isWithinRoot(curDir, mountRoot) {
		return "", "", fmt.Errorf("cgroup v2 path %q escapes mount root %q", curDir, mountRoot)
	}
	return curDir, mountRoot, nil
}

// getMemoryLimitV2 resolves the process's cgroup v2 directory and walks up to the cgroup2
// mount root taking the smallest numeric memory.max limit (treating "max" as unlimited at
// that level, matching automemlimit semantics).
func getMemoryLimitV2(procCgroupFile, procMountinfoFile, fallbackMountRoot string) (int64, error) {
	curDir, mountRoot, err := resolveCgroupV2Dir(procCgroupFile, procMountinfoFile, fallbackMountRoot)
	if err != nil {
		return 0, err
	}

	var minLimit int64 = math.MaxInt64
	found := false

	for {
		data, readErr := os.ReadFile(filepath.Join(curDir, "memory.max"))
		if readErr == nil {
			val := strings.TrimSpace(string(data))
			if val != "" && val != "max" {
				if n, parseErr := strconv.ParseInt(val, 10, 64); parseErr == nil && n > 0 && n < minLimit {
					minLimit = n
					found = true
				}
			}
		}
		if curDir == mountRoot {
			break
		}
		parent := filepath.Dir(curDir)
		if parent == curDir || !isWithinRoot(parent, mountRoot) {
			break
		}
		curDir = parent
	}

	if !found {
		return 0, fmt.Errorf("no numeric memory.max limit found in cgroup v2 chain under %q", mountRoot)
	}
	return minLimit, nil
}

func getMemoryUsageV2(procCgroupFile, procMountinfoFile, fallbackMountRoot string) (int64, error) {
	curDir, _, err := resolveCgroupV2Dir(procCgroupFile, procMountinfoFile, fallbackMountRoot)
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(filepath.Join(curDir, "memory.current"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

func parseCgroupV2Path(procCgroupFile string) (string, error) {
	raw, err := os.ReadFile(procCgroupFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::"), nil
		}
	}
	return "", fmt.Errorf("cgroup v2 entry (0::) not found in %q", procCgroupFile)
}

func parseCgroupV2Mounts(procMountinfoFile string) []cgroupV2Mount {
	raw, err := os.ReadFile(procMountinfoFile)
	if err != nil {
		return nil
	}
	var mounts []cgroupV2Mount
	for _, line := range strings.Split(string(raw), "\n") {
		idx := strings.Index(line, " - ")
		if idx < 0 {
			continue
		}
		left := strings.Fields(line[:idx])
		right := strings.Fields(line[idx+3:])
		if len(left) >= 5 && len(right) >= 1 && right[0] == "cgroup2" {
			mounts = append(mounts, cgroupV2Mount{
				root:       unescapeMountinfoPath(left[3]),
				mountPoint: unescapeMountinfoPath(left[4]),
			})
		}
	}
	return mounts
}

func unescapeMountinfoPath(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
