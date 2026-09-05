package worker

import (
	"context"
	"errors"
	"fmt"
)

// Service owns the lifetime of the normal Consumer and the PEL Reclaimer.
type Service struct {
	consumer  *Consumer
	reclaimer *Reclaimer
}

// NewService assembles Worker components without launching background work.
func NewService(consumer *Consumer, reclaimer *Reclaimer) (*Service, error) {
	if consumer == nil {
		return nil, fmt.Errorf("consumer is required")
	}
	if reclaimer == nil {
		return nil, fmt.Errorf("reclaimer is required")
	}
	return &Service{consumer: consumer, reclaimer: reclaimer}, nil
}

// Run starts both loops and cancels the sibling loop when either one exits.
func (service *Service) Run(ctx context.Context) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- service.consumer.Run(child) }()
	go func() { done <- service.reclaimer.Run(child) }()
	first := <-done
	cancel()
	second := <-done
	if first != nil && !errors.Is(first, context.Canceled) {
		return first
	}
	if second != nil && !errors.Is(second, context.Canceled) {
		return second
	}
	return ctx.Err()
}
