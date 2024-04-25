//go:build !nobench
// +build !nobench

package gogctuner

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync/atomic"
	"testing"
	"time"
)

type treeNode struct {
	Children []*treeNode
}

func generateTree(nChild, depth int) *treeNode {
	if depth == 0 {
		return &treeNode{}
	}
	ret := &treeNode{}
	ret.Children = make([]*treeNode, 0, nChild)
	for i := 0; i < nChild; i++ {
		ret.Children = append(ret.Children, generateTree(nChild, depth-1))
	}
	return ret
}

func TestBenchGcTuner(t *testing.T) {
	enableProfiler := true
	defaultRounds := doBench("default", t, enableProfiler)
	// enable adaptive gc
	configurator := newConfigSupplier()
	configurator.SetConfig(Config{MaxRAMPercentage: 10})

	_ = EnableGCTuner(WithConfigurator(configurator))

	/*
		go func() {
			time.Sleep(time.Minute)
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				configurator.config.MaxRAMPercentage = 0
				configurator.config.GOGC += 100
				ch <- struct{}{}
			}
		}()
	*/
	adativeRounds := doBench("adaptive", t, enableProfiler)

	// reset adaptive gc
	configurator.SetConfig(Config{})

	log.Printf("adaptive rounds: %d, default rounds: %d, improvement: %0.2f%%", adativeRounds, defaultRounds,
		(float64(adativeRounds)-float64(defaultRounds))/float64(defaultRounds)*100)
}

func doBench(tag string, t *testing.T, enableProfiler bool) uint64 {
	defer func() {
		if r := recover(); r != nil {
			t.Error("The code paniced", r)
		}
	}()
	beforeStats := &debug.GCStats{}
	debug.ReadGCStats(beforeStats)
	//installGcHookOnce.Do(installGCHook)
	if enableProfiler {
		filename := fmt.Sprintf("%s_cpu.pprof", tag)
		_ = os.RemoveAll(filename)
		f, _ := os.Create(filename)
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	start := time.Now()
	watchDuration := 20 * time.Second
	var i uint64
	go func() {
		for time.Since(start) < watchDuration {
			m := generateTree(3, 12)
			atomic.AddUint64(&i, 1)
			_ = m
		}
	}()
	time.Sleep(watchDuration + time.Second*3)
	stats := &debug.GCStats{}
	debug.ReadGCStats(stats)
	log.Printf("====================")
	log.Printf("total rounds for %s: %d", tag, atomic.LoadUint64(&i))
	log.Printf("gc summary: NumGC %d, PauseTotal %v",
		stats.NumGC-beforeStats.NumGC, stats.PauseTotal-beforeStats.PauseTotal)
	log.Printf("====================")
	return atomic.LoadUint64(&i)
}

func installGCHook() {
	var f = &ref{}
	runtime.SetFinalizer(f, gcInfoPrinter)
	f = nil
}

func gcInfoPrinter(f *ref) {
	stats := &debug.GCStats{}
	debug.ReadGCStats(stats)
	log.Printf("gc triggered at %v, %d GCs, pause %v", stats.LastGC, stats.NumGC, stats.Pause[0])
	runtime.SetFinalizer(f, gcInfoPrinter)
}

func newConfigSupplier() *configSupplier {
	supplier := &configSupplier{
		ch: make(chan interface{}),
	}
	return supplier
}

type configSupplier struct {
	config atomic.Value
	ch     chan interface{}
}

func (c *configSupplier) Updates() <-chan interface{} {
	return c.ch
}

func (c *configSupplier) GetConfig() (Config, error) {
	config, _ := c.config.Load().(Config)
	return config, nil
}

func (c *configSupplier) SetConfig(config Config) {
	c.config.Store(config)
	select {
	case c.ch <- struct{}{}: // notify the config change
	default:
	}
}
