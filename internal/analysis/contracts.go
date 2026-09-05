package analysis

import "time"

// SnapshotRecord is the read-side representation used by the analysis UI.
// Failed tasks intentionally keep their error fields while Payload stays empty.
type SnapshotRecord struct {
	TaskID      uint64     `json:"task_id"`
	RuleID      string     `json:"rule_id"`
	Status      string     `json:"status"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	FailureCode string     `json:"failure_code,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	Payload     string     `json:"payload,omitempty"`
}

// RuleDefinition tells the frontend how to interpret one rule's JSON result.
// It is metadata, not an online rule editor or an execution permission model.
type RuleDefinition struct {
	Script   string            `json:"script,omitempty" yaml:"script"`
	URL      string            `json:"url,omitempty" yaml:"url"`
	ID       string            `json:"id" yaml:"id"`
	Name     string            `json:"name" yaml:"name"`
	Source   string            `json:"source" yaml:"source"`
	Schedule string            `json:"schedule" yaml:"schedule"`
	MinItems int               `json:"min_items" yaml:"min_items"`
	ItemKey  string            `json:"item_key" yaml:"item_key"`
	Display  map[string]string `json:"display" yaml:"display"`
}
