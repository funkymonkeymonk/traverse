package provider

// Registry manages provider plugins
type Registry struct {
	providers map[string]Factory
}

// Factory creates new provider instances
type Factory func() Provider

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Factory),
	}
}

// Register adds a provider factory to the registry
func (r *Registry) Register(name string, factory Factory) {
	r.providers[name] = factory
}

// Create instantiates a provider by name
func (r *Registry) Create(name string) (Provider, error) {
	factory, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return factory(), nil
}

// List returns all registered provider names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
