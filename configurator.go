package gogctuner

import (
	"log"
	"sync/atomic"
)

func NewGcConfigurator() *gcConfigurator {
	return &gcConfigurator{
		configUpdateCh: make(chan interface{}, 1),
	}
}

type gcConfigurator struct {
	config         atomic.Value
	configUpdateCh chan interface{}
}

func (g *gcConfigurator) GetConfig() (Config, error) {
	c, _ := g.config.Load().(Config)
	return c, nil
}

func (g *gcConfigurator) Updates() <-chan interface{} {
	return g.configUpdateCh
}

func (g *gcConfigurator) SetConfig(value Config) {
	g.config.Store(value)

	select {
	case g.configUpdateCh <- struct{}{}:
		log.Printf("gctuner: gc config update event dispatched, config: %v", value)
	default:
		log.Printf("gctuner: gc config update event is dropped due to previous task not done, config: %v", value)
	}
}
