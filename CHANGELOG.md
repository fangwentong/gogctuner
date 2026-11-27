# Changelog

## 0.2.0

### Breaking Changes
- Minimum Go version bumped from Go 1.12 to **Go 1.21**.
- Removed pre-Go 1.19 dynamic `GOGC` fallback path (`adjustGOGCByMemoryLimit`, `GetLiveDatasetSize`) and per-GC
  finalizer hook.

### Fixes & Improvements
- Fixed initialization crash: `internal/memory` no longer panics when system memory cannot be determined;
  `EnableGCTuner` returns an error cleanly.
- Fixed nested cgroup v2 limit detection: resolves `/proc/self/cgroup` (`0::<path>`) and `/proc/self/mountinfo`,
  walking up the ancestor chain to take the minimum numeric `memory.max` limit (treating `"max"` as unlimited at that
  level).
- Exported `GcConfigurator` and removed hardcoded global `log.Printf` calls inside `SetConfig`.
- Added unit tests for config state transitions, `GOMEMLIMIT` env restoration, dynamic updates under `-race`, and
  cgroup v2 ancestor-chain fixtures.
- Modernized GitHub Actions CI (`ubuntu-24.04`, `macos-14`, `windows-latest`, Go 1.21/1.25/1.26, `-race`,
  `golangci-lint`).
