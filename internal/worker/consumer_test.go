package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestConsumerRunReadErrors(t *testing.T) {
	for _, alreadyCancelled := range []bool{false, true} {
		name := "closed client error is returned"
		if alreadyCancelled {
			name = "caller cancellation takes precedence"
		}
		t.Run(name, func(t *testing.T) {
			client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
			cfg := DefaultConfig("test-stream", "test-group", "test-consumer")
			processor, err := NewProcessor(client, cfg)
			if err != nil {
				t.Fatal(err)
			}
			consumer, err := NewConsumer(client, cfg, processor)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			want := redis.ErrClosed
			if alreadyCancelled {
				cancel()
				want = context.Canceled
			}
			if err := consumer.Run(ctx); !errors.Is(err, want) {
				t.Fatalf("Run error = %v, want %v", err, want)
			}
		})
	}
}
