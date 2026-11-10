package plugin

import "context"

// Middleware is one stage of a waterfall. Calling next delegates to the next
// listener; returning without calling it stops the chain.
type Middleware func(ctx context.Context, payload any, next func(context.Context) (any, error)) (any, error)

// OnMiddleware registers a waterfall stage on this context.
func (c *Context) OnMiddleware(name string, middleware Middleware) Listener {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.middlewares[name] = append(c.middlewares[name], middlewareEntry{id: id, middleware: middleware})
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		list := c.middlewares[name]
		for i, entry := range list {
			if entry.id == id {
				c.middlewares[name] = append(list[:i:i], list[i+1:]...)
				return
			}
		}
	}
}

// Waterfall runs the registered middleware around an optional terminal handler,
// mirroring the Cordis events whose listeners must call next() to delegate.
// Middleware registered on a child context runs before its ancestors'.
func (c *Context) Waterfall(ctx context.Context, name string, payload any, terminal Handler) (any, error) {
	var chain []Middleware
	for node := c; node != nil; node = node.parent {
		node.mu.RLock()
		entries := append([]middlewareEntry(nil), node.middlewares[name]...)
		node.mu.RUnlock()
		for _, entry := range entries {
			chain = append(chain, entry.middleware)
		}
	}

	var run func(context.Context, int) (any, error)
	run = func(ctx context.Context, index int) (any, error) {
		if index >= len(chain) {
			if terminal != nil {
				terminal(ctx, payload)
			}
			return payload, nil
		}
		return chain[index](ctx, payload, func(nextCtx context.Context) (any, error) {
			return run(nextCtx, index+1)
		})
	}
	return run(ctx, 0)
}
