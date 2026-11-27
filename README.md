# gogctuner: Dynamic GC Tuner for Go Applications

`gogctuner` is a lightweight Go library (Go 1.21+) that automatically configures Go's soft memory limit
(`debug.SetMemoryLimit`) and `GOGC` (`debug.SetGCPercent`) based on a target percentage of container/host RAM, with
support for **runtime dynamic configuration updates** (`Configurator`).

## Features

- **Container-Aware Memory Limit**: Reads cgroup v2 (`memory.max` via `/proc/self/cgroup` + `/proc/self/mountinfo`
  ancestor-chain minimum, matching `automemlimit` semantics) and cgroup v1 (`memory.limit_in_bytes` /
  `hierarchical_memory_limit`), falling back to host RAM.
- **Dynamic Hot Reload**: Plug in any config center via the `Configurator` interface (`GetConfig` + `Updates` channel)
  to adjust `MaxRAMPercentage` or `GOGC` at runtime without restarting the service.
- **Zero Per-GC Overhead**: Purely event-driven on Go 1.21+; no per-GC finalizer hooks.
- **Cross-OS Compatibility**: Runs cleanly on Linux, macOS, and Windows.

## Important Sizing Guidance

> [!WARNING]
> When `MaxRAMPercentage > 0` and `GOGC == 0` (the default "unset" sentinel), `gogctuner` sets `GOGC=-1` so GC is
> governed solely by `GOMEMLIMIT`. Because container memory limits also include non-Go-heap memory (cgo allocations,
> `mmap`, page cache, runtime metadata), setting `MaxRAMPercentage` close to `100` risks OOM kills. In production
> containers, **80%–90%** (or lower for cgo-heavy workloads) is recommended.

## `Config.GOGC` Semantics

| `MaxRAMPercentage` | `GOGC` | Behavior |
| --- | --- | --- |
| `> 0` | `0` (unset) | Sets `GOMEMLIMIT = MaxRAMPercentage% * memLimit` and `GOGC = -1` |
| `> 0` | `!= 0` | Sets both `GOMEMLIMIT` and `debug.SetGCPercent(GOGC)` |
| `== 0` | `0` (unset) | Resets `GOMEMLIMIT` to `GOMEMLIMIT` env (or unlimited) and `GOGC` to `GOGC` env (or `100`) |
| `== 0` | `< 0` / `> 0` | Sets `debug.SetGCPercent(GOGC)` directly |

## Usage

### Static Configuration

```go
package main

import (
	"log"

	"github.com/fangwentong/gogctuner"
)

func main() {
	if err := gogctuner.EnableGCTuner(
		gogctuner.WithStaticConfig(gogctuner.Config{MaxRAMPercentage: 80}),
	); err != nil {
		log.Fatalf("enable gctuner: %v", err)
	}
}
```

### Dynamic Configuration

```go
cfg := gogctuner.NewGcConfigurator()
cfg.SetConfig(gogctuner.Config{MaxRAMPercentage: 80})

if err := gogctuner.EnableGCTuner(gogctuner.WithConfigurator(cfg)); err != nil {
	log.Fatalf("enable gctuner: %v", err)
}

// Update at runtime from a config center watcher:
cfg.SetConfig(gogctuner.Config{MaxRAMPercentage: 70})
```

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
