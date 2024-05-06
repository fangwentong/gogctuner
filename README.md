# gogctuner: GC Tuner for Go Applications

`gogctuner` is a Go library designed to automatically adjust the garbage collection (GC) parameters to optimize CPU
usage based on the target memory usage percentage.

## How it works

- **Pre Go1.19 Versions**: Utilizes a tuning strategy inspired
  by [Uber's article](https://www.uber.com/blog/how-we-saved-70k-cores-across-30-mission-critical-services/), which
  adjusts the `GOGC` parameter based on the live heap size and target memory limit.
- **Go1.19 and Later**: Takes advantage of
  the [soft memory limit feature](https://github.com/golang/proposal/blob/master/design/48409-soft-memory-limit.md)
  introduced in Go 1.19, offering more granular control over GC behavior.

## Features

- **Memory Usage Based Tuning**: Automatically adjusts GC parameters to maintain a specified percentage of memory usage.
- **Cross-Version Compatibility**: Compatible with Go versions below 1.19 and leverages the `SetMemoryLimit` features in
  Go 1.19 and above.
- **cgroups Support**: Works seamlessly with both cgroups and cgroupsv2 for enhanced resource management.
- **Cross-OS Compatibility**: Ensures functionality across multiple operating systems.

## Usage

### Static Configuration

Use static configuration by setting the `MaxRAMPercentage` at the initialization of your application:

```go
package main

import (
  "github.com/fangwentong/gogctuner"
)

func main() {
  gogctuner.EnableGCTuner(
    gogctuner.WithStaticConfig(gogctuner.Config{MaxRAMPercentage: 90}),
  )
  // Your application code here
}

```

### Dynamic Configuration

For dynamic configuration that allows runtime updates, you can use a configurator:

```go
package main

import (
  "github.com/fangwentong/gogctuner"
)

func main() {
  configurator := gogctuner.NewGcConfigurator()

  // Integrate with your dynamic config implementation here:
  conf := readFromYourConfigCenter("your_config_key")
  configurator.SetConfig(conf) // set initial config
  // register config updates callback
  registerConfigUpdateCallback("your_config_key", func(conf gogctuner.Config) {
    configurator.SetConfig(conf)
  })

  gogctuner.EnableGCTuner(
    gogctuner.WithConfigurator(configurator),
  )
  // Your application code here
}

```

### Reference

- Golang GC Guide: https://tip.golang.org/doc/gc-guide
- How We Saved 70K Cores Across 30 Mission-Critical Services (Large-Scale, Semi-Automated Go GC Tuning
  @Uber): https://www.uber.com/blog/how-we-saved-70k-cores-across-30-mission-critical-services/
- https://github.com/cch123/gogctuner
- https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/lib/cgroup

### License

This project is licensed under the MIT License, see the [LICENSE](LICENSE) file for details.

