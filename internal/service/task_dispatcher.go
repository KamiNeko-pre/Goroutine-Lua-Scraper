package service

import (
	"context"
	"fmt"
	"time"

	"go-lua-crawler/internal/queue"
	"go-lua-crawler/internal/repository"
	taskmodel "go-lua-crawler/internal/task"
)

func EnqueueCrawlTask(ctx context.Context, taskID uint64) (string, error) {
	task, err := repository.GetCrawlTask(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("load task: %w", err)
	}
	if task.Status != taskmodel.StatusPending {
		return "", fmt.Errorf("task is not pending: %s", task.Status)
	}

	messageID, err := queue.PublishTask(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("publish task: %w", err)
	}

	if err := repository.TransitionCrawlTask(
		ctx,
		taskID,
		taskmodel.StatusPending,
		taskmodel.StatusQueued,
		map[string]any{"queued_at": time.Now()},
	); err != nil {
		return messageID, fmt.Errorf("mark task queued: %w", err)
	}

	return messageID, nil
}
