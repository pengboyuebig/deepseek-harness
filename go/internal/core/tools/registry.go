// Package tools is the scoped tool registry and execution pipeline, the Go
// counterpart of packages/core/tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Definition is the model-facing schema of one tool plus its executor.
type Definition struct {
	Name        string                                                           `json:"name"`
	Description string                                                           `json:"description"`
	Parameters  json.RawMessage                                                  `json:"parameters,omitempty"`
	Execute     func(ctx context.Context, input json.RawMessage) (string, error) `json:"-"`
}

// Registry holds the tools visible to one agent scope.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Definition
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Definition)}
}

// Register adds a tool; a duplicate name fails so composition errors surface at
// boot rather than at call time.
func (r *Registry) Register(definition Definition) error {
	if definition.Name == "" {
		return fmt.Errorf("tools: tool has no name")
	}
	if definition.Execute == nil {
		return fmt.Errorf("tools: tool %q has no executor", definition.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[definition.Name]; exists {
		return fmt.Errorf("tools: tool %q already registered", definition.Name)
	}
	r.tools[definition.Name] = definition
	return nil
}

// Schemas returns the definitions ordered by name for prompt assembly.
func (r *Registry) Schemas() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Definition, 0, len(r.tools))
	for _, definition := range r.tools {
		out = append(out, definition)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Call runs a tool through the pre-execute → execute → post-execute pipeline.
func (r *Registry) Call(ctx context.Context, name string, input json.RawMessage) (string, error) {
	r.mu.RLock()
	definition, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tools: unknown tool %q", name)
	}
	return definition.Execute(ctx, input)
}
