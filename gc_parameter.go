package gogctuner

import (
	"math"
	"reflect"
)

// setGCParameter applies GC parameter transitions using Go 1.19+ GOMEMLIMIT and GOGC APIs.
func setGCParameter(oldConfig, newConfig Config, logger Logger, hooks runtimeHooks) error {
	if reflect.DeepEqual(oldConfig, newConfig) {
		return nil
	}
	if newConfig.MaxRAMPercentage > 0 {
		memLimit, err := hooks.getMemoryLimit()
		if err != nil {
			logger.Errorf("gctuner: failed to adjust GC, get memory limit err: %v", err)
			return err
		}
		limit := int64(newConfig.MaxRAMPercentage / 100.0 * float64(memLimit))
		gogc := newConfig.GOGC
		if gogc == 0 {
			gogc = -1 // Disable percentage-based GC unless explicitly specified.
		}
		// Set memory limit BEFORE adjusting GOGC (especially GOGC=-1) so GC is never momentarily unbounded.
		hooks.setMemoryLimit(limit)
		hooks.setGCPercent(gogc)
		logger.Logf("gctuner: set memory limit %v (GOGC=%d)", printMemorySize(uint64(limit)), gogc)
		return nil
	}

	if oldConfig.MaxRAMPercentage != 0 {
		// Restore GOGC BEFORE releasing GOMEMLIMIT back to default/unlimited so GC is never momentarily disabled.
		setGOGCOrDefault(newConfig.GOGC, logger, hooks)
		defaultMemLimit := readGOMEMLIMIT(hooks.getEnv, logger)
		logger.Logf("gctuner: reset memory limit %v", printMemorySize(uint64(defaultMemLimit)))
		hooks.setMemoryLimit(defaultMemLimit)
		return nil
	}

	if newConfig.GOGC != oldConfig.GOGC {
		setGOGCOrDefault(newConfig.GOGC, logger, hooks)
	}
	return nil
}

// readGOMEMLIMIT reads the GOMEMLIMIT value from the environment.
// Copied from runtime.readGOMEMLIMIT.
func readGOMEMLIMIT(getEnv func(string) string, logger Logger) int64 {
	p := getEnv("GOMEMLIMIT")
	if p == "" || p == "off" {
		return math.MaxInt64
	}
	n, ok := parseByteCount(p)
	if !ok {
		logger.Errorf("GOMEMLIMIT=%s", p)
		logger.Errorf("malformed GOMEMLIMIT; see `go doc runtime/debug.SetMemoryLimit`")
		return math.MaxInt64
	}
	return n
}

// parseByteCount parses a string that represents a count of bytes.
//
// s must match the following regular expression:
//
//	^[0-9]+(([KMGT]i)?B)?$
//
// Copied from runtime.parseByteCount.
func parseByteCount(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	last := s[len(s)-1]
	if last >= '0' && last <= '9' {
		n, ok := atoi64(s)
		if !ok || n < 0 {
			return 0, false
		}
		return n, ok
	}
	if last != 'B' || len(s) < 2 {
		return 0, false
	}
	if c := s[len(s)-2]; c >= '0' && c <= '9' {
		n, ok := atoi64(s[:len(s)-1])
		if !ok || n < 0 {
			return 0, false
		}
		return n, ok
	} else if c != 'i' {
		return 0, false
	}
	if len(s) < 4 {
		return 0, false
	}
	power := 0
	switch s[len(s)-3] {
	case 'K':
		power = 1
	case 'M':
		power = 2
	case 'G':
		power = 3
	case 'T':
		power = 4
	default:
		return 0, false
	}
	m := uint64(1)
	for i := 0; i < power; i++ {
		m *= 1024
	}
	n, ok := atoi64(s[:len(s)-3])
	if !ok || n < 0 {
		return 0, false
	}
	un := uint64(n)
	if un > math.MaxInt64/m {
		return 0, false
	}
	un *= m
	if un > uint64(math.MaxInt64) {
		return 0, false
	}
	return int64(un), true
}

func atoi64(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}

	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}

	un := uint64(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		if un > math.MaxUint64/10 {
			return 0, false
		}
		un *= 10
		un1 := un + uint64(c) - '0'
		if un1 < un {
			return 0, false
		}
		un = un1
	}

	if !neg && un > uint64(math.MaxInt64) {
		return 0, false
	}
	if neg && un > uint64(math.MaxInt64)+1 {
		return 0, false
	}

	n := int64(un)
	if neg {
		n = -n
	}

	return n, true
}
