package tasks

import (
	"context"
	"fmt"
	"sync"
)

// TaskHandler is the function signature for a task handler
// It takes context and arguments, and returns a result map and error
type TaskHandler func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)

// Registry stores the mapping of task names to handlers
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]TaskHandler
}

// GlobalRegistry is the default global registry
var GlobalRegistry = &Registry{
	handlers: make(map[string]TaskHandler),
}

// Register adds a handler for a task name
func (r *Registry) Register(name string, handler TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get retrieves a handler for a task name
func (r *Registry) Get(name string) (TaskHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[name]
	return handler, ok
}

// RegisterHandler is a helper to register to the global registry
func RegisterHandler(name string, handler TaskHandler) {
	GlobalRegistry.Register(name, handler)
}

// GetHandler is a helper to get from the global registry
func GetHandler(name string) (TaskHandler, bool) {
	return GlobalRegistry.Get(name)
}

// Initialize registers default tasks (can be expanded)
func Initialize() {
	// Example task
	RegisterHandler("example_task", func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		fmt.Printf("Executing example_task with args: %v\n", args)
		return map[string]interface{}{"status": "success", "message": "Example task executed"}, nil
	})
}
