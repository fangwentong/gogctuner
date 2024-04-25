package gogctuner

import (
	"fmt"
	"log"
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
	// Config is the configuration for adaptive GC
	Config struct {
		// GOGC is the target GOGC value if specified,
		// It is effective when MaxRAMPercentage == 0,
		// the default value is the value read from the `GOGC` environment variable,
		// or 100 if the `GOGC` environment variable is not set.
		// See debug.SetGCPercent(GOGC)
		GOGC int `json:"gogc,omitempty" yaml:"gogc,omitempty"`

		// MaxRAMPercentage is the maximum memory usage, range (0, 100]
		MaxRAMPercentage float64 `json:"max_ram_percentage,omitempty" yaml:"max_ram_percentage,omitempty"`
	}

	// Configurator is an interface for configuration management
	Configurator interface {
		// GetConfig retrieves the gctuner Config
		GetConfig() (Config, error)

		// Updates is the channel of config update events, which will be triggered upon config updates.
		// It returns nil if the configurator does not support config updates.
		// The gctuner will obtain the latest config by calling GetConfig.
		Updates() <-chan interface{}
	}

	Logger interface {
		Logf(format string, v ...interface{})
		Errorf(format string, v ...interface{})
	}
)

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

// WithConfigurator sets the configurator for gctuner
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

// WithLogger sets the logger for gctuner, if not specified, a stdout logger is used by default.
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
		return fmt.Errorf("invalid max_ram_percentage value: %f, expected range (0, 100]", c.MaxRAMPercentage)
	}
	return nil
}

func doInit(options ...Option) error {
	o := &opts{}
	for _, opt := range options {
		opt(o)
	}

	if o.logger == nil {
		o.logger = &stdLogger{}
	}

	if o.configurator == nil {
		o.logger.Errorf("error while init gctuner: %v", errNoConfiguratorSpecified)
		return errNoConfiguratorSpecified
	}

	h := &adaptiveGCHandler{
		configurator: o.configurator,
		logger:       o.logger,
		ch:           make(chan interface{}, 1),
	}
	h.Start()
	return nil
}

type adaptiveGCHandler struct {
	configurator Configurator
	logger       Logger

	prevConfig atomic.Value
	ch         chan interface{}
}

func (a *adaptiveGCHandler) Start() {
	a.checkAndSetNextGCConfig()
	a.installGCHook()
	go a.handleConfigTask()
	go a.withRecover(a.watchConfigUpdate)()
}

func (a *adaptiveGCHandler) installGCHook() {
	var r = &ref{}
	runtime.SetFinalizer(r, a.registerNextGCHook)
	r = nil
}

func (a *adaptiveGCHandler) checkAndSetNextGCConfig() {
	newConfig, err := a.configurator.GetConfig()
	if err != nil {
		a.logger.Errorf("get gc config error: %v", err)
		return
	}
	if err = newConfig.CheckValid(); err != nil {
		a.logger.Errorf("check gc config error: %v", err)
		return
	}

	oldConfig, _ := a.prevConfig.Load().(Config)
	setGCParameter(oldConfig, newConfig, a.logger)
	a.prevConfig.Store(newConfig)
}

func (a *adaptiveGCHandler) watchConfigUpdate() {
	configUpdateCh := a.configurator.Updates()
	if configUpdateCh == nil {
		return
	}
	for range configUpdateCh {
		newVal, _ := a.configurator.GetConfig()
		oldVal, _ := a.prevConfig.Load().(Config)
		if reflect.DeepEqual(newVal, oldVal) {
			// The config has not changed
			continue
		}
		// The config has changed
		select {
		case a.ch <- struct{}{}:
		default:
		}
	}
}

func (a *adaptiveGCHandler) handleConfigTask() {
	for range a.ch {
		a.withRecover(a.checkAndSetNextGCConfig)()
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

func (a *adaptiveGCHandler) registerNextGCHook(f *ref) {
	select {
	case a.ch <- struct{}{}:
	default:
	}
	runtime.SetFinalizer(f, a.registerNextGCHook) // Keep the object f alive for the next GC
}

type ref struct {
	holder []byte
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
