package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLuaParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := executeLuaScriptContext(ctx, "while true do end", "https://example.com", time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
