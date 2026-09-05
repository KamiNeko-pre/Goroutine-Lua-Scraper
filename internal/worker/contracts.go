package worker

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// MessageProcessor owns task-state decisions after Redis delivers a message.
// New and reclaimed messages are intentionally separate because their valid
// MySQL starting states are different.
type MessageProcessor interface {
	ProcessNewMessage(ctx context.Context, message redis.XMessage) error
	ProcessClaimedMessage(ctx context.Context, message redis.XMessage) error
}
