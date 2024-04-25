//go:build !go1.19
// +build !go1.19

package gogctuner

// setGCParameter set GC parameters
func setGCParameter(oldConfig, newConfig Config, logger Logger) {
	adjustGOGCByMemoryLimit(oldConfig, newConfig, logger)
}
