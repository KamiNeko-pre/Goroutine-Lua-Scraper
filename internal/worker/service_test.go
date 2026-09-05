package worker

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestServiceStopsSiblingAfterReadFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	client.Close()
	cfg := DefaultConfig("s", "g", "c")
	p, _ := NewProcessor(client, cfg)
	c, _ := NewConsumer(client, cfg, p)
	r, _ := NewReclaimer(client, cfg, p)
	s, _ := NewService(c, r)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Run(ctx); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("error=%v", err)
	}
}
