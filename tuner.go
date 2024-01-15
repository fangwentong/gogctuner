package gogctuner

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
)

func init() {
	logger = &StdLoggerAdapter{}
}

type ILogger interface {
	Error(args ...interface{})
	Debug(args ...interface{})
}
type OptFunc func() error

func SetLogger(l ILogger) OptFunc {
	return func() error {
		if l != nil {
			logger = l
		}
		return nil
	}
}

type StdLoggerAdapter struct {
	DebugEnabled bool
}

func (l *StdLoggerAdapter) Error(args ...interface{}) {
	log.Print(args...)
}

func (l *StdLoggerAdapter) Debug(args ...interface{}) {
	if l.DebugEnabled {
		log.Print(args...)
	}
}

type finalizer struct {
	ref *finalizerRef
}

type finalizerRef struct {
	parent *finalizer
}

// default GOGC = 100
var previousGOGC = 100

// don't trigger err log on every failure
var failCounter = -1

var logger ILogger

func getCurrentPercentAndChangeGOGC() {
	memPercent, err := getUsage()

	if err != nil {
		failCounter++
		if failCounter%10 == 0 {
			logger.Error(fmt.Sprintf("gctuner: failed to adjust GC err :%v", err.Error()))
		}
		return
	}

	newgogc := getGOGC(previousGOGC, memoryLimitInPercent, memPercent)

	if previousGOGC != newgogc {
		logger.Debug(fmt.Sprintf("gctuner: current mem usage %%:%v. adjusting GOGC - from %v to %v", memPercent, previousGOGC, newgogc))
		previousGOGC = debug.SetGCPercent(newgogc)
	}
}

func getGOGC(previousGOGC int, memoryLimitInPercent, memPercent float64) int {
	// hard_target = live_dataset + live_dataset * (GOGC / 100).
	// 	hard_target =  memoryLimitInPercent
	// 	live_dataset = memPercent
	//  so gogc = (hard_target - livedataset) / live_dataset * 100
	newgogc := (memoryLimitInPercent - memPercent) / memPercent * 100.0

	// if newgogc < 0, we have to use the previous gogc to determine the next
	if newgogc < 0 {
		newgogc = float64(previousGOGC) * memoryLimitInPercent / memPercent
	}

	return int(newgogc)
}

func finalizerHandler(f *finalizerRef) {
	getCurrentPercentAndChangeGOGC()
	runtime.SetFinalizer(f, finalizerHandler)
}

// NewTuner
//   set useCgroup to true if your app is in docker
//   set percent to control the gc trigger, 0-100, 100 or upper means no limit
//
//   modify default GOGC value in the case there's an env variable set.
func NewTuner(useCgroup bool, percent float64, options ...OptFunc) *finalizer {
	errs := []error{}
	for _, opt := range options {
		err := opt()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		for _, v := range errs {
			logger.Error(v)
		}
	}

	if envGOGC := os.Getenv("GOGC"); envGOGC != "" {
		n, err := strconv.Atoi(envGOGC)
		if err == nil {
			previousGOGC = n
		}
	}
	if useCgroup {
		getUsage = getUsageCGroup
	} else {
		getUsage = getUsageNormal
	}

	memoryLimitInPercent = percent

	logger.Debug(fmt.Sprintf("gctuner: GC Tuner initialized. GOGC: %v Target percentage: %v", previousGOGC, percent))

	f := &finalizer{}

	f.ref = &finalizerRef{parent: f}
	runtime.SetFinalizer(f.ref, finalizerHandler)
	f.ref = nil
	return f
}
