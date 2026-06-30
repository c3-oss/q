package adapter

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds a factory under each of its schemes. Engine subpackages call
// this from init(); a duplicate scheme panics, which surfaces a programming
// error at startup rather than at query time.
func Register(f Factory) {
	mu.Lock()
	defer mu.Unlock()
	for _, scheme := range f.Schemes() {
		if _, dup := registry[scheme]; dup {
			panic(fmt.Sprintf("adapter: scheme %q registered twice", scheme))
		}
		registry[scheme] = f
	}
}

// Lookup returns the factory registered for a scheme.
func Lookup(scheme string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[scheme]
	return f, ok
}

// Schemes returns every registered scheme, sorted, for diagnostics.
func Schemes() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for s := range registry {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
