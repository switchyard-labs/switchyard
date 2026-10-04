package app

import (
	"context"
	"errors"
	"testing"
)

func TestCancelledReconciliationDoesNotContactDependencies(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.reconcileOnceContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
