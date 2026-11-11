// Package plugin is the Go replacement for the vendored Cordis context:
// a tree of services, typed events, and reversible effects.
package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Handler receives one event payload.
type Handler func(context.Context, any)

// Listener removes a registered handler when called.
type Listener func()

// Context is one node of the plugin tree. A child context inherits its
// parent's services and listeners and owns its own effects.
type Context struct {
	parent      *Context
	log         *slog.Logger
	mu          sync.RWMutex
	services    map[string]any
	handlers    map[string][]handlerEntry
	middlewares map[string][]middlewareEntry
	nextID      uint64
	effects     []func()
	children    []*Context
	disposed    bool
}

// New creates a root context.
func New(log *slog.Logger) *Context {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Context{
		log:         log,
		services:    make(map[string]any),
		handlers:    make(map[string][]handlerEntry),
		middlewares: make(map[string][]middlewareEntry),
	}
}

// Child creates a scoped context. Disposing the parent disposes its children
// first, so a scoped plugin can never outlive the context that created it.
func (c *Context) Child() *Context {
	child := &Context{
		parent:      c,
		log:         c.log,
		services:    make(map[string]any),
		handlers:    make(map[string][]handlerEntry),
		middlewares: make(map[string][]middlewareEntry),
	}
	c.mu.Lock()
	c.children = append(c.children, child)
	c.mu.Unlock()
	return child
}

// Logger returns the context logger.
func (c *Context) Logger() *slog.Logger { return c.log }

// Provide registers a service under name. Registering the same key twice is an
// error so a conflicting provider fails at boot rather than at first use.
func (c *Context) Provide(name string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.services[name]; exists {
		return fmt.Errorf("plugin: service %q already provided", name)
	}
	c.services[name] = value
	return nil
}

// Get resolves a service from this context or its ancestors.
func (c *Context) Get(name string) (any, bool) {
	c.mu.RLock()
	value, ok := c.services[name]
	c.mu.RUnlock()
	if ok {
		return value, true
	}
	if c.parent != nil {
		return c.parent.Get(name)
	}
	return nil, false
}

// MustGet resolves a service or panics; use it only during composition.
func (c *Context) MustGet(name string) any {
	value, ok := c.Get(name)
	if !ok {
		panic(fmt.Sprintf("plugin: service %q is not registered", name))
	}
	return value
}

type handlerEntry struct {
	id      uint64
	handler Handler
}

type middlewareEntry struct {
	id         uint64
	middleware Middleware
}

// On registers a typed event handler.
func (c *Context) On(name string, handler Handler) Listener {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.handlers[name] = append(c.handlers[name], handlerEntry{id: id, handler: handler})
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		list := c.handlers[name]
		for i, entry := range list {
			if entry.id == id {
				c.handlers[name] = append(list[:i:i], list[i+1:]...)
				return
			}
		}
	}
}

func (c *Context) removeChild(child *Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, candidate := range c.children {
		if candidate == child {
			c.children = append(c.children[:i:i], c.children[i+1:]...)
			return
		}
	}
}

// Emit delivers payload to every handler registered on this context and its
// ancestors, newest registration last.
func (c *Context) Emit(ctx context.Context, name string, payload any) {
	for node := c; node != nil; node = node.parent {
		node.mu.RLock()
		entries := append([]handlerEntry(nil), node.handlers[name]...)
		node.mu.RUnlock()
		for _, entry := range entries {
			entry.handler(ctx, payload)
		}
	}
}

// Effect records a reversible side effect. Effects run LIFO on Dispose.
func (c *Context) Effect(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.effects = append(c.effects, fn)
}

// Dispose unwinds effects in reverse registration order. It is idempotent.
func (c *Context) Dispose() error {
	c.mu.Lock()
	if c.disposed {
		c.mu.Unlock()
		return nil
	}
	c.disposed = true
	effects := c.effects
	children := c.children
	c.effects = nil
	c.children = nil
	c.services = make(map[string]any)
	c.handlers = make(map[string][]handlerEntry)
	c.middlewares = make(map[string][]middlewareEntry)
	c.mu.Unlock()

	var first error
	for i := len(children) - 1; i >= 0; i-- {
		if err := children[i].Dispose(); err != nil && first == nil {
			first = err
		}
	}
	for i := len(effects) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil && first == nil {
					first = fmt.Errorf("plugin: effect panic: %v", recovered)
				}
			}()
			effects[i]()
		}()
	}
	return first
}
