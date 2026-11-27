package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetHierarchicalMemoryLimitSuccess(t *testing.T) {
	result, err := getHierarchicalMemoryLimit("./testdata/cgroup", "./testdata/self/cgroup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(120)
	if result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestGetHierarchicalMemoryLimitFailure(t *testing.T) {
	_, err := getHierarchicalMemoryLimit("./testdata/not_exist", "./testdata/self/not_exist")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestGetMemoryLimitV2AncestorChain(t *testing.T) {
	t.Run("isolated namespace 0::/", func(t *testing.T) {
		root := t.TempDir()
		cgRoot := filepath.Join(root, "sys", "fs", "cgroup")
		writeFixtureFile(t, filepath.Join(cgRoot, "memory.max"), "1073741824\n")
		procCg := filepath.Join(root, "proc_cgroup")
		procMount := filepath.Join(root, "proc_mountinfo")
		writeFixtureFile(t, procCg, "0::/\n")
		writeFixtureFile(t, procMount, fmt.Sprintf("29 22 0:26 / /%s rw,nosuid - cgroup2 cgroup2 rw\n", cgRoot[1:]))

		got, err := getMemoryLimitV2(procCg, procMount, "/nonexistent")
		if err != nil || got != 1073741824 {
			t.Fatalf("expected 1073741824, got %d (err=%v)", got, err)
		}
	})

	t.Run("nested child limit with root max", func(t *testing.T) {
		root := t.TempDir()
		cgRoot := filepath.Join(root, "cgroup2")
		writeFixtureFile(t, filepath.Join(cgRoot, "memory.max"), "max\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "kubepods.slice", "memory.max"), "max\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "kubepods.slice", "pod1", "memory.max"), "536870912\n")
		procCg := filepath.Join(root, "proc_cgroup")
		procMount := filepath.Join(root, "proc_mountinfo")
		writeFixtureFile(t, procCg, "0::/kubepods.slice/pod1\n")
		writeFixtureFile(t, procMount, fmt.Sprintf("29 22 0:26 / %s rw - cgroup2 cgroup2 rw\n", cgRoot))

		got, err := getMemoryLimitV2(procCg, procMount, "/nonexistent")
		if err != nil || got != 536870912 {
			t.Fatalf("expected 536870912, got %d (err=%v)", got, err)
		}
	})

	t.Run("parent limit smaller than child", func(t *testing.T) {
		root := t.TempDir()
		cgRoot := filepath.Join(root, "cgroup2")
		writeFixtureFile(t, filepath.Join(cgRoot, "memory.max"), "max\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "kubepods.slice", "memory.max"), "268435456\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "kubepods.slice", "pod1", "memory.max"), "536870912\n")
		procCg := filepath.Join(root, "proc_cgroup")
		procMount := filepath.Join(root, "proc_mountinfo")
		writeFixtureFile(t, procCg, "0::/kubepods.slice/pod1\n")
		writeFixtureFile(t, procMount, fmt.Sprintf("29 22 0:26 / %s rw - cgroup2 cgroup2 rw\n", cgRoot))

		got, err := getMemoryLimitV2(procCg, procMount, "/nonexistent")
		if err != nil || got != 268435456 {
			t.Fatalf("expected parent min 268435456, got %d (err=%v)", got, err)
		}
	})

	t.Run("all max falls back with error", func(t *testing.T) {
		root := t.TempDir()
		cgRoot := filepath.Join(root, "cgroup2")
		writeFixtureFile(t, filepath.Join(cgRoot, "memory.max"), "max\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "kubepods.slice", "memory.max"), "max\n")
		procCg := filepath.Join(root, "proc_cgroup")
		procMount := filepath.Join(root, "proc_mountinfo")
		writeFixtureFile(t, procCg, "0::/kubepods.slice\n")
		writeFixtureFile(t, procMount, fmt.Sprintf("29 22 0:26 / %s rw - cgroup2 cgroup2 rw\n", cgRoot))

		got, err := getMemoryLimitV2(procCg, procMount, "/nonexistent")
		if err == nil || got != 0 {
			t.Fatalf("expected error and 0 when all levels are max, got %d (err=%v)", got, err)
		}
	})

	t.Run("non-root mountinfo root prefix and octal space escape", func(t *testing.T) {
		root := t.TempDir()
		cgRoot := filepath.Join(root, "cg mount")
		writeFixtureFile(t, filepath.Join(cgRoot, "memory.max"), "max\n")
		writeFixtureFile(t, filepath.Join(cgRoot, "app", "memory.max"), "134217728\n")
		procCg := filepath.Join(root, "proc_cgroup")
		procMount := filepath.Join(root, "proc_mountinfo")
		writeFixtureFile(t, procCg, "0::/lxc/container1/app\n")
		escapedMount := filepath.Join(root, `cg\040mount`)
		writeFixtureFile(t, procMount, fmt.Sprintf("29 22 0:26 /lxc/container1 %s rw - cgroup2 cgroup2 rw\n", escapedMount))

		got, err := getMemoryLimitV2(procCg, procMount, "/nonexistent")
		if err != nil || got != 134217728 {
			t.Fatalf("expected 134217728, got %d (err=%v)", got, err)
		}
	})
}
