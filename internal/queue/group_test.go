package queue

import (
	"context"
	"testing"
)

func TestEnsureConsumerGroupRejectsUninitializedClient(t *testing.T) {
	previousClient := Client
	Client = nil
	t.Cleanup(func() { Client = previousClient })

	if err := EnsureConsumerGroup(context.Background()); err == nil {
		t.Fatal("expected an error when Redis client is not initialized")
	}
}
