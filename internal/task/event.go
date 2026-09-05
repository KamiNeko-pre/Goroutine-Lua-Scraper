package task

import "time"

const (
	EventStageAccepted        = "accepted"
	EventStageOutboxCreated   = "outbox_created"
	EventStagePublished       = "published"
	EventStageClaimed         = "claimed"
	EventStageLuaFinished     = "lua_finished"
	EventStageResultValidated = "result_validated"
	EventStageMySQLCommitted  = "mysql_committed"
	EventStageCompleted       = "completed"
	EventStageFailed          = "failed"
	EventStageAcked           = "acked"
)

const (
	EventLevelInfo  = "info"
	EventLevelError = "error"
)

// TaskEvent stores only key lifecycle milestones for one crawl task.
// It is a trace index for diagnosis, not a replacement for process logs.
type TaskEvent struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID    uint64    `json:"task_id" gorm:"index;not null"`
	Stage     string    `json:"stage" gorm:"type:varchar(32);not null"`
	Level     string    `json:"level" gorm:"type:varchar(16);not null"`
	Code      string    `json:"code,omitempty" gorm:"type:varchar(64)"`
	Message   string    `json:"message,omitempty" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}
