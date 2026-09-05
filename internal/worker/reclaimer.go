package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Reclaimer periodically adopts messages left in the Pending Entries List by
// Consumers that stopped before acknowledging them.
type Reclaimer struct {
	client    *redis.Client
	config    Config
	processor MessageProcessor
}

// NewReclaimer validates dependencies without starting a goroutine.
func NewReclaimer(client *redis.Client, cfg Config, processor MessageProcessor) (*Reclaimer, error) {
	if client == nil {
		return nil, fmt.Errorf("Redis client is required")
	}
	if processor == nil {
		return nil, fmt.Errorf("message processor is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate reclaimer config: %w", err)
	}
	return &Reclaimer{client: client, config: cfg, processor: processor}, nil
}

// Run blocks until ctx is cancelled or reclaiming/processing a message fails.
func (reclaimer *Reclaimer) Run(ctx context.Context) error {
	ticker := time.NewTicker(reclaimer.config.ClaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			cursor := "0-0"
			for {
				messages, next, err := reclaimer.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
					Stream: reclaimer.config.Stream, Group: reclaimer.config.Group,
					Consumer: reclaimer.config.Consumer, MinIdle: reclaimer.config.ClaimMinIdle,
					Start: cursor, Count: reclaimer.config.ReadCount,
				}).Result()
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if err != nil && !errors.Is(err, redis.Nil) {
					return fmt.Errorf("claim pending messages: %w", err)
				}
				for _, message := range messages {
					if err := reclaimer.processor.ProcessClaimedMessage(ctx, message); err != nil {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						log.Printf("reclaimed message %s still pending: %v", message.ID, err)
					}
				}
				if next == "0-0" || next == "" {
					break
				}
				cursor = next
			}
		}
	}
}
