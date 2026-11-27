package gogctuner

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

var (
	initOnce                   sync.Once
	initError                  error
	errNoConfiguratorSpecified = fmt.Errorf("no gctuner configurator specified")
)

type (
	// Config is the configuration for adaptive GC.
	Config struct {
		// GOGC configures the Go runtime GC target percentage.
		// When MaxRAMPercentage == 0:
		//   - GOGC == 0 resets GOGC to the process default (from the GOGC env var, or 100).
		//   - GOGC < 0 disables percentage-based GC (GOGC=off).
		//   - GOGC > 0 sets debug.SetGCPercent(GOGC).
		// When MaxRAMPercentage > 0:
		//   - GOGC == 0 is treated as "not specified" and sets GOGC=-1 so GC is driven
		//     by GOMEMLIMIT alone.
		//   - GOGC != 0 applies both debug.SetGCPercent(GOGC) and debug.SetMemoryLimit.
		GOGC int `json:"gogc,omitempty" yaml:"gogc,omitempty"`

		// MaxRAMPercentage is the target soft memory limit as a percentage of container/host RAM, in [0, 100].
		// Values above 80-90% are generally discouraged in containers because non-Go-heap memory
		// (cgo, mmap, runtime overhead) still counts toward the container memory limit.
		MaxRAMPercentage float64 `json:"max_ram_percentage,omitempty" yaml:"max_ram_percentage,omitempty"`
	}

	// Configurator is an interface for configuration management.
	Configurator interface {
		// GetConfig retrieves the latest gctuner Config.
		GetConfig() (Config, error)

		// Updates is the channel of config update events, triggered upon config updates.
		// It returns nil if the configurator does not support dynamic updates.
		Updates() <-chan interface{}
	}

	Logger interface {
		Logf(format string, v ...interface{})
		Errorf(format string, v ...interface{})
	}
)

// runtimeHooks abstracts runtime/OS interactions for unit testing without widening the public API.
type runtimeHooks struct {
	setGCPercent   func(int) int
	setMemoryLimit func(int64) int64
	getMemoryLimit func() (uint64, error)
	getEnv         func(string) string
}

func defaultRuntimeHooks() runtimeHooks {
	return runtimeHooks{
		setGCPercent:   debug.SetGCPercent,
		setMemoryLimit: debug.SetMemoryLimit,
		getMemoryLimit: getMemoryLimit,
		getEnv:         os.Getenv,
	}
}

func EnableGCTuner(options ...Option) error {
	initOnce.Do(func() {
		initError = doInit(options...)
	})
	return initError
}

type opts struct {
	logger       Logger
	configurator Configurator
}

type Option func(*opts)

// WithConfigurator sets the configurator for gctuner.
func WithConfigurator(configurator Configurator) Option {
	return func(o *opts) {
		o.configurator = configurator
	}
}

// WithStaticConfig sets a static config for gctuner, which will not be updated.
func WithStaticConfig(config Config) Option {
	return func(o *opts) {
		o.configurator = staticConfigurator{config: config}
	}
}

// WithLogger sets the logger for gctuner; if not specified, a stdlib log logger is used.
func WithLogger(logger Logger) Option {
	return func(o *opts) {
		o.logger = logger
	}
}

func (c *Config) CheckValid() error {
	if c == nil {
		return nil
	}
	if c.MaxRAMPercentage < 0 || c.MaxRAMPercentage > 100 {
		return fmt.Errorf("invalid max_ram_percentage value: %f, expected range [0, 100]", c.MaxRAMPercentage)
	}
	return nil
}

func doInit(options ...Option) error {
	h, err := newAdaptiveGCHandler(defaultRuntimeHooks(), options...)
	if err != nil {
		return err
	}
	return h.Start()
}

func newAdaptiveGCHandler(hooks runtimeHooks, options ...Option) (*adaptiveGCHandler, error) {
	o := &opts{}
	for _, opt := range options {
		opt(o)
	}

	if o.logger == nil {
		o.logger = &stdLogger{}
	}

	if o.configurator == nil {
		o.logger.Errorf("error while init gctuner: %v", errNoConfiguratorSpecified)
		return nil, errNoConfiguratorSpecified
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &adaptiveGCHandler{
		configurator: o.configurator,
		logger:       o.logger,
		hooks:        hooks,
		ch:           make(chan struct{}, 1),
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

type adaptiveGCHandler struct {
	configurator Configurator
	logger       Logger
	hooks        runtimeHooks

	prevConfig atomic.Value
	ch         chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (a *adaptiveGCHandler) Start() error {
	if err := a.applyConfigOnce(); err != nil {
		a.cancel()
		return err
	}
	if a.configurator.Updates() == nil {
		return nil
	}
	a.wg.Add(2)
	go func() {
		defer a.wg.Done()
		a.handleConfigTask()
	}()
	go func() {
		defer a.wg.Done()
		a.withRecover(a.watchConfigUpdate)()
	}()
	return nil
}

func (a *adaptiveGCHandler) stop() {
	a.cancel()
	a.wg.Wait()
}

func (a *adaptiveGCHandler) applyConfigOnce() (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			stackStr := string(debug.Stack())
			a.logger.Errorf("applyConfigOnce panic: %v, stack: %s", r, stackStr)
			retErr = fmt.Errorf("gctuner: panic applying config: %v", r)
		}
	}()

	newConfig, err := a.configurator.GetConfig()
	if err != nil {
		a.logger.Errorf("get gc config error: %v", err)
		return err
	}
	if err = newConfig.CheckValid(); err != nil {
		a.logger.Errorf("check gc config error: %v", err)
		return err
	}

	oldConfig, _ := a.prevConfig.Load().(Config)
	if err = setGCParameter(oldConfig, newConfig, a.logger, a.hooks); err != nil {
		return err
	}
	a.prevConfig.Store(newConfig)
	return nil
}

func (a *adaptiveGCHandler) watchConfigUpdate() {
	configUpdateCh := a.configurator.Updates()
	if configUpdateCh == nil {
		return
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case _, ok := <-configUpdateCh:
			if !ok {
				return
			}
			// Always signal handleConfigTask without deduplicating against prevConfig here;
			// setGCParameter performs authoritative deduplication on the single worker goroutine.
			select {
			case a.ch <- struct{}{}:
			default:
			}
		}
	}
}

func (a *adaptiveGCHandler) handleConfigTask() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.ch:
			_ = a.applyConfigOnce()
		}
	}
}

func (a *adaptiveGCHandler) withRecover(func0 func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				stackStr := string(debug.Stack())
				a.logger.Errorf("%s panic: %v, stack: %s",
					runtime.FuncForPC(reflect.ValueOf(func0).Pointer()).Name(), r, stackStr)
			}
		}()
		func0()
	}
}

type stdLogger struct{}

func (l *stdLogger) Logf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *stdLogger) Errorf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

type staticConfigurator struct {
	config Config
}

func (s staticConfigurator) Updates() <-chan interface{} {
	return nil
}

func (s staticConfigurator) GetConfig() (Config, error) {
	return s.config, nil
}
