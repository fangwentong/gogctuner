//go:build go1.19
// +build go1.19

package gogctuner

import (
	"math"
	"os"
	"reflect"
	"runtime/debug"
)

// setGCParameter sets the GC parameters
func setGCParameter(oldConfig, newConfig Config, logger Logger) {
	if reflect.DeepEqual(oldConfig, newConfig) {
		// The config has no change
		return
	}
	if newConfig.MaxRAMPercentage > 0 {
		memLimit, err := getMemoryLimit()
		if err != nil {
			logger.Errorf("gctuner: failed to adjust GC, get memory limit err: %v", err.Error())
			return
		}
		limit := int64(newConfig.MaxRAMPercentage / 100.0 * float64(memLimit))
		gogc := newConfig.GOGC
		if gogc == 0 { // gogc is not set
			gogc = -1 // Disable GC unless the memory limit is reached
		}
		debug.SetGCPercent(gogc)
		debug.SetMemoryLimit(limit)
		logger.Logf("gctuner: set memory limit %v", printMemorySize(uint64(limit)))
		return
	}

	if oldConfig.MaxRAMPercentage != 0 {
		// The config has been changed, reset the memory limit and GOGC
		defaultMemLimit := readGOMEMLIMIT(logger)
		if defaultMemLimit != 0 {
			logger.Logf("gctuner: reset memory limit %v", printMemorySize(uint64(defaultMemLimit)))
			debug.SetMemoryLimit(defaultMemLimit)
		}
		// reset GOGC
		setGOGCOrDefault(newConfig.GOGC, logger)
		return
	}

	// Set the GOGC value
	if newConfig.GOGC != oldConfig.GOGC {
		setGOGCOrDefault(newConfig.GOGC, logger)
	}
}

// readGOMEMLIMIT reads the GOMEMLIMIT value
// Copied from runtime.readGOMEMLIMIT
func readGOMEMLIMIT(logger Logger) int64 {
	p := os.Getenv("GOMEMLIMIT")
	if p == "" || p == "off" {
		return math.MaxInt64
	}
	n, ok := parseByteCount(p)
	if !ok {
		// shouldn't be here, the `runtime.readGOMEMLIMIT` would exit the process in advance
		logger.Errorf("GOMEMLIMIT=", p, "\n")
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
// In other words, an integer byte count with an optional unit
// suffix. Acceptable suffixes include one of
// - KiB, MiB, GiB, TiB which represent binary IEC/ISO 80000 units, or
// - B, which just represents bytes.
//
// Returns an int64 because that's what its callers want and receive,
// but the result is always non-negative.
// Copied from runtime.parseByteCount
func parseByteCount(s string) (int64, bool) {
	// The empty string is not valid.
	if s == "" {
		return 0, false
	}
	// Handle the easy non-suffix case.
	last := s[len(s)-1]
	if last >= '0' && last <= '9' {
		n, ok := atoi64(s)
		if !ok || n < 0 {
			return 0, false
		}
		return n, ok
	}
	// Failing a trailing digit, this must always end in 'B'.
	// Also at this point there must be at least one digit before
	// that B.
	if last != 'B' || len(s) < 2 {
		return 0, false
	}
	// The one before that must always be a digit or 'i'.
	if c := s[len(s)-2]; c >= '0' && c <= '9' {
		// Trivial 'B' suffix.
		n, ok := atoi64(s[:len(s)-1])
		if !ok || n < 0 {
			return 0, false
		}
		return n, ok
	} else if c != 'i' {
		return 0, false
	}
	// Finally, we need at least 4 characters now, for the unit
	// prefix and at least one digit.
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
		// Invalid suffix.
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
		// Overflow.
		return 0, false
	}
	un *= m
	if un > uint64(math.MaxInt64) {
		// Overflow.
		return 0, false
	}
	return int64(un), true
}

// atoi64 parses an int64 from a string s.
// The bool result reports whether s is a number
// representable by a value of type int64.
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
			// overflow
			return 0, false
		}
		un *= 10
		un1 := un + uint64(c) - '0'
		if un1 < un {
			// overflow
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
