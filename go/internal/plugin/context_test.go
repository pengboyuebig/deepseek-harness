// Package plugin_test covers the context lifecycle contract: LIFO unwinding,
// scoped children, and listener removal.
package plugin_test

import (
	"context"
	"errors"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

func TestEffectsUnwindLastInFirstOut(t *testing.T) {
	ctx := plugin.New(nil)
	var order []string
	ctx.Effect(func() { order = append(order, "first") })
	ctx.Effect(func() { order = append(order, "second") })
	ctx.Effect(func() { order = append(order, "third") })

	if err := ctx.Dispose(); err != nil {
		t.Fatalf("dispose: %v", err)
	}
	want := []string{"third", "second", "first"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestDisposeIsIdempotentAndReportsPanics(t *testing.T) {
	ctx := plugin.New(nil)
	ctx.Effect(func() { panic("boom") })
	ctx.Effect(func() {})

	if err := ctx.Dispose(); err == nil {
		t.Fatal("expected the panic to surface as an error")
	}
	if err := ctx.Dispose(); err != nil {
		t.Fatalf("second dispose = %v, want nil", err)
	}
}

func TestChildSeesParentServicesButDisposesAlone(t *testing.T) {
	root := plugin.New(nil)
	if err := root.Provide("config", 42); err != nil {
		t.Fatalf("provide: %v", err)
	}
	if err := root.Provide("config", 43); err == nil {
		t.Fatal("duplicate service must fail")
	}

	child := root.Child()
	value, ok := child.Get("config")
	if !ok || value != 42 {
		t.Fatalf("child lookup = %v, %v", value, ok)
	}
	if err := child.Provide("scoped", true); err != nil {
		t.Fatalf("child provide: %v", err)
	}

	disposed := false
	child.Effect(func() { disposed = true })
	if err := child.Dispose(); err != nil {
		t.Fatalf("child dispose: %v", err)
	}
	if !disposed {
		t.Fatal("child effect did not run")
	}
	if _, ok := root.Get("scoped"); ok {
		t.Fatal("child service leaked into the parent")
	}
	if _, ok := root.Get("config"); !ok {
		t.Fatal("root service disappeared after child dispose")
	}
}

func TestHandlersRunAndCanBeRemoved(t *testing.T) {
	ctx := plugin.New(nil)
	var seen []string
	off := ctx.On("turn/start", func(_ context.Context, payload any) {
		seen = append(seen, payload.(string))
	})

	ctx.Emit(context.Background(), "turn/start", "one")
	off()
	ctx.Emit(context.Background(), "turn/start", "two")

	if len(seen) != 1 || seen[0] != "one" {
		t.Fatalf("seen = %v, want [one]", seen)
	}
}

func TestWaterfallStopsWhenNextIsNotCalled(t *testing.T) {
	ctx := plugin.New(nil)
	ctx.OnMiddleware("agent/pre-step", func(_ context.Context, payload any, _ func(context.Context) (any, error)) (any, error) {
		return payload, errors.New("rejected")
	})

	_, err := ctx.Waterfall(context.Background(), "agent/pre-step", "messages", nil)
	if err == nil || err.Error() != "rejected" {
		t.Fatalf("err = %v, want rejected", err)
	}
}

func TestWaterfallDelegatesThroughTheChain(t *testing.T) {
	ctx := plugin.New(nil)
	var order []string
	ctx.OnMiddleware("agent/request", func(ctx context.Context, payload any, next func(context.Context) (any, error)) (any, error) {
		order = append(order, "first")
		return next(ctx)
	})
	ctx.OnMiddleware("agent/request", func(ctx context.Context, payload any, next func(context.Context) (any, error)) (any, error) {
		order = append(order, "second")
		return next(ctx)
	})

	if _, err := ctx.Waterfall(context.Background(), "agent/request", nil, nil); err != nil {
		t.Fatalf("waterfall: %v", err)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("order = %v, want [first second]", order)
	}
}

func TestDisposingTheParentDisposesItsChildren(t *testing.T) {
	root := plugin.New(nil)
	child := root.Child()
	grandchild := child.Child()
	var order []string
	root.Effect(func() { order = append(order, "root") })
	child.Effect(func() { order = append(order, "child") })
	grandchild.Effect(func() { order = append(order, "grandchild") })

	if err := root.Dispose(); err != nil {
		t.Fatalf("dispose: %v", err)
	}
	want := []string{"grandchild", "child", "root"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
