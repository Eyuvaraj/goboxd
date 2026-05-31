package registry

import (
	"sync"
	"time"
)

const probeTTL = 30 * time.Second

// ProbeCache caches probe results with a probeTTL refresh. Prevents spawning
// child processes on every /readyz or /info call.
type ProbeCache struct {
	mu        sync.RWMutex
	nsjail    ProbeResult
	languages map[string]ProbeResult
	at        time.Time

	reg        *Registry
	nsjailPath string
	done       chan struct{} // closed by Stop() to terminate the background loop
	stopOnce   sync.Once
}

func NewProbeCache(reg *Registry, nsjailPath string) *ProbeCache {
	pc := &ProbeCache{
		reg:        reg,
		nsjailPath: nsjailPath,
		done:       make(chan struct{}),
	}
	pc.refresh()
	go pc.loop()
	return pc
}

// Stop terminates the background refresh goroutine. Safe to call multiple times.
func (pc *ProbeCache) Stop() {
	pc.stopOnce.Do(func() { close(pc.done) })
}

// Nsjail returns the cached nsjail probe result.
func (pc *ProbeCache) Nsjail() ProbeResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nsjail
}

// Languages returns a copy of the cached language probe results.
func (pc *ProbeCache) Languages() map[string]ProbeResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	out := make(map[string]ProbeResult, len(pc.languages))
	for k, v := range pc.languages {
		out[k] = v
	}
	return out
}

// AllOK returns true when nsjail and every language probe passed.
func (pc *ProbeCache) AllOK() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if !pc.nsjail.OK {
		return false
	}
	for _, r := range pc.languages {
		if !r.OK {
			return false
		}
	}
	return true
}

func (pc *ProbeCache) loop() {
	ticker := time.NewTicker(probeTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pc.refresh()
		case <-pc.done:
			return
		}
	}
}

func (pc *ProbeCache) refresh() {
	nsjail := ProbeNsjail(pc.nsjailPath)
	langs := make(map[string]ProbeResult, pc.reg.Len())
	for _, lang := range pc.reg.All() {
		langs[lang.ID] = ProbeLanguage(lang)
	}
	pc.mu.Lock()
	pc.nsjail = nsjail
	pc.languages = langs
	pc.at = time.Now()
	pc.mu.Unlock()
}
