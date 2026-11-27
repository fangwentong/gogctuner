package gogctuner

import "sync/atomic"

// GcConfigurator is a thread-safe dynamic Configurator implementation.
type GcConfigurator struct {
	config         atomic.Value
	configUpdateCh chan interface{}
}

// NewGcConfigurator creates a new GcConfigurator for dynamic GC config updates.
func NewGcConfigurator() *GcConfigurator {
	return &GcConfigurator{
		configUpdateCh: make(chan interface{}, 1),
	}
}

func (g *GcConfigurator) GetConfig() (Config, error) {
	c, _ := g.config.Load().(Config)
	return c, nil
}

func (g *GcConfigurator) Updates() <-chan interface{} {
	return g.configUpdateCh
}

func (g *GcConfigurator) SetConfig(value Config) {
	g.config.Store(value)

	select {
	case g.configUpdateCh <- struct{}{}:
	default:
	}
}
