package gogctuner

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

type noopLogger struct{}

func (noopLogger) Logf(string, ...interface{})   {}
func (noopLogger) Errorf(string, ...interface{}) {}

type mockRuntime struct {
	mu          sync.Mutex
	gogcCalls   []int
	memCalls    []int64
	callOrder   []string
	memLimit    uint64
	memLimitErr error
	env         map[string]string
}

func (m *mockRuntime) hooks() runtimeHooks {
	return runtimeHooks{
		setGCPercent: func(v int) int {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.gogcCalls = append(m.gogcCalls, v)
			m.callOrder = append(m.callOrder, "gogc")
			return 100
		},
		setMemoryLimit: func(v int64) int64 {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.memCalls = append(m.memCalls, v)
			m.callOrder = append(m.callOrder, "mem")
			return math.MaxInt64
		},
		getMemoryLimit: func() (uint64, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.memLimit, m.memLimitErr
		},
		getEnv: func(k string) string {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.env[k]
		},
	}
}

func TestGcConfig(t *testing.T) {
	cases := []struct {
		pct   float64
		valid bool
	}{
		{0, true},
		{80, true},
		{100, true},
		{101, false},
		{-1, false},
	}
	for _, tc := range cases {
		cfg := Config{MaxRAMPercentage: tc.pct}
		if (cfg.CheckValid() == nil) != tc.valid {
			t.Errorf("MaxRAMPercentage=%f expected valid=%v", tc.pct, tc.valid)
		}
	}
}

func TestSetGCParameterTransitions(t *testing.T) {
	const gib = 1 << 30

	t.Run("static GOGC to MaxRAMPercentage to static reset with env GOMEMLIMIT", func(t *testing.T) {
		mr := &mockRuntime{
			memLimit: 4 * gib,
			env:      map[string]string{"GOMEMLIMIT": "512MiB", "GOGC": "150"},
		}
		// 1) Initial static GOGC
		if err := setGCParameter(Config{}, Config{GOGC: 200}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(mr.gogcCalls) != 1 || mr.gogcCalls[0] != 200 || len(mr.memCalls) != 0 {
			t.Fatalf("unexpected calls after static GOGC: gogc=%v mem=%v", mr.gogcCalls, mr.memCalls)
		}

		// 2) Switch to MaxRAMPercentage=75 with GOGC=0 -> GOGC=-1 & limit=3GiB
		if err := setGCParameter(Config{GOGC: 200}, Config{MaxRAMPercentage: 75}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if mr.gogcCalls[len(mr.gogcCalls)-1] != -1 || mr.memCalls[len(mr.memCalls)-1] != 3*gib {
			t.Fatalf("unexpected calls after MaxRAMPercentage: gogc=%v mem=%v", mr.gogcCalls, mr.memCalls)
		}

		// 3) Switch to MaxRAMPercentage=50 with explicit GOGC=100 -> both applied
		if err := setGCParameter(Config{MaxRAMPercentage: 75}, Config{MaxRAMPercentage: 50, GOGC: 100}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if mr.gogcCalls[len(mr.gogcCalls)-1] != 100 || mr.memCalls[len(mr.memCalls)-1] != 2*gib {
			t.Fatalf("unexpected calls with explicit GOGC: gogc=%v mem=%v", mr.gogcCalls, mr.memCalls)
		}

		// 4) Reset back to zero config -> restores env GOMEMLIMIT=512MiB and env GOGC=150
		if err := setGCParameter(Config{MaxRAMPercentage: 50, GOGC: 100}, Config{}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if mr.memCalls[len(mr.memCalls)-1] != 512<<20 || mr.gogcCalls[len(mr.gogcCalls)-1] != 150 {
			t.Fatalf("unexpected reset calls: gogc=%v mem=%v", mr.gogcCalls, mr.memCalls)
		}
	})

	t.Run("reset restores unlimited when GOMEMLIMIT unset or off", func(t *testing.T) {
		mr := &mockRuntime{memLimit: 2 * gib, env: map[string]string{}}
		if err := setGCParameter(Config{MaxRAMPercentage: 80}, Config{}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if mr.memCalls[0] != math.MaxInt64 || mr.gogcCalls[0] != 100 {
			t.Fatalf("expected MaxInt64 & 100, got mem=%v gogc=%v", mr.memCalls, mr.gogcCalls)
		}
		if len(mr.callOrder) != 2 || mr.callOrder[0] != "gogc" || mr.callOrder[1] != "mem" {
			t.Fatalf("expected reset order [gogc mem], got %v", mr.callOrder)
		}
	})

	t.Run("enable sets memory limit before disabling GOGC", func(t *testing.T) {
		mr := &mockRuntime{memLimit: 2 * gib, env: map[string]string{}}
		if err := setGCParameter(Config{}, Config{MaxRAMPercentage: 80}, noopLogger{}, mr.hooks()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(mr.callOrder) != 2 || mr.callOrder[0] != "mem" || mr.callOrder[1] != "gogc" {
			t.Fatalf("expected enable order [mem gogc], got %v", mr.callOrder)
		}
	})

	t.Run("memory limit error returned without panic", func(t *testing.T) {
		mr := &mockRuntime{memLimitErr: errors.New("boom"), env: map[string]string{}}
		h, err := newAdaptiveGCHandler(mr.hooks(), WithStaticConfig(Config{MaxRAMPercentage: 70}), WithLogger(noopLogger{}))
		if err != nil {
			t.Fatalf("newAdaptiveGCHandler err: %v", err)
		}
		if err := h.Start(); err == nil {
			t.Fatalf("expected error when memory limit fails")
		}
	})
}

func TestDynamicConfiguratorUpdatesAndCleanStop(t *testing.T) {
	const gib = 1 << 30
	mr := &mockRuntime{memLimit: 4 * gib, env: map[string]string{}}
	cfg := NewGcConfigurator()
	cfg.SetConfig(Config{MaxRAMPercentage: 50})

	h, err := newAdaptiveGCHandler(mr.hooks(), WithConfigurator(cfg), WithLogger(noopLogger{}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := h.Start(); err != nil {
		t.Fatalf("start err: %v", err)
	}
	defer h.stop()

	cfg.SetConfig(Config{MaxRAMPercentage: 75})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mr.mu.Lock()
		n := len(mr.memCalls)
		last := int64(0)
		if n > 0 {
			last = mr.memCalls[n-1]
		}
		mr.mu.Unlock()
		if n >= 2 && last == 3*gib {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for dynamic config update; memCalls=%v", mr.memCalls)
}
