package notification

import (
	"sync"
)

// Registry manages notification provider factories
type Registry struct {
	factories map[string]func() Notifier
	mu        sync.RWMutex
}

// NewRegistry creates a new notification registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]func() Notifier),
	}
}

// Register adds a notifier factory to the registry
func (r *Registry) Register(name string, factory func() Notifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Create instantiates a notifier by name
func (r *Registry) Create(name string) (Notifier, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrNotifierNotFound
	}

	return factory(), nil
}

// List returns all registered notifier names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}

	return names
}

// Global registry instance
var globalRegistry = NewRegistry()

// Register adds a notifier factory to the global registry
func Register(name string, factory func() Notifier) {
	globalRegistry.Register(name, factory)
}

// CreateNotifier creates a notifier from the global registry
func CreateNotifier(name string) (Notifier, error) {
	return globalRegistry.Create(name)
}

// ListNotifiers returns all registered notifier names from the global registry
func ListNotifiers() []string {
	return globalRegistry.List()
}
