package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrHandlerNotRegistered is returned by Registry.Lookup when no Handler
// has been registered for a job type.
var ErrHandlerNotRegistered = errors.New("job: handler not registered")

// Handler executes one job's payload and returns its result. Implementations
// must respect ctx cancellation/deadline: the worker pool uses it to enforce
// per-job timeouts and to abandon work cleanly on shutdown.
type Handler interface {
	Execute(ctx context.Context, payload json.RawMessage) (result json.RawMessage, err error)
}

// HandlerFunc adapts a plain function to the Handler interface.
type HandlerFunc func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error)

// Execute calls f.
func (f HandlerFunc) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	return f(ctx, payload)
}

// Registry maps a job type name to the Handler that executes it. A single
// Registry is shared read-mostly across all workers in a pool, so lookups
// must be safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register associates jobType with h, panicking if jobType is already
// registered. Registration happens once at process startup, so a duplicate
// registration is a programming error, not a runtime condition to recover
// from.
func (r *Registry) Register(jobType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[jobType]; exists {
		panic(fmt.Sprintf("job: handler already registered for type %q", jobType))
	}
	r.handlers[jobType] = h
}

// Lookup returns the Handler registered for jobType, or ErrHandlerNotRegistered.
func (r *Registry) Lookup(jobType string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrHandlerNotRegistered, jobType)
	}
	return h, nil
}

// Types returns the registered job type names.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
