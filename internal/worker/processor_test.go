package worker

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestNewProcessorDependencies(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	t.Cleanup(func() { _ = client.Close() })
	cfg := DefaultConfig("test-stream", "test-group", "test-consumer")
	if _, err := NewProcessor(nil, cfg); err == nil {
		t.Fatal("nil client must be rejected")
	}
	if _, err := NewProcessor(client, Config{}); err == nil {
		t.Fatal("invalid config must be rejected")
	}
	processor, err := NewProcessor(client, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if processor.client != client || processor.config != cfg {
		t.Fatal("processor did not retain its Redis dependencies")
	}
}
