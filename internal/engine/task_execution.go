package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go-lua-crawler/internal/config"
	"go-lua-crawler/internal/repository"
	taskmodel "go-lua-crawler/internal/task"
	"gorm.io/gorm"
)

// ExecuteTask reuses the existing VM/parser but defers persistence to completion.
// The Channel entrypoint keeps its original persistence behavior.
func ExecuteTask(ctx context.Context, task taskmodel.CrawlTask) (repository.PersistTaskResult, error) {
	ScriptMutex.RLock()
	script := ScriptCache
	ScriptMutex.RUnlock()
	// Configured targets select a trusted local rule; legacy callers keep the cache.
	if rules, loadErr := repository.ListRuleDefinitions(); loadErr == nil {
		for _, rule := range rules {
			if rule.ID == task.Target && rule.Script != "" {
				content, readErr := os.ReadFile(rule.Script)
				if readErr != nil {
					return nil, fmt.Errorf("load rule %s: %w", task.Target, readErr)
				}
				script = string(content)
				break
			}
		}
	}
	if script == "" {
		return nil, fmt.Errorf("no Lua rule loaded")
	}
	result, err := executeLuaScriptContext(ctx, script, task.URL, time.Duration(config.Get().Engine.LuaTimeout)*time.Second)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return func(tx *gorm.DB) error {
		return tx.Create(&repository.CrawlResult{TaskID: task.ID, Payload: string(payload)}).Error
	}, nil
}
