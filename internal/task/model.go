package task

import "time"

// CrawlTask records one accepted collection request. MySQL stores this record as
// the source of truth while Redis Streams will later transport only its ID.
type CrawlTask struct {
	ID          uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestKey  string     `json:"request_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	Target      string     `json:"target" gorm:"type:varchar(64);not null"`
	URL         string     `json:"url" gorm:"type:text;not null"`
	Status      Status     `json:"status" gorm:"type:varchar(20);not null;index"`
	RetryCount  int        `json:"retry_count" gorm:"not null;default:0"`
	RunToken    string     `json:"-" gorm:"type:varchar(32);not null;default:''"`
	LeaseUntil  *time.Time `json:"lease_until,omitempty" gorm:"index"`
	FailureCode string     `json:"failure_code,omitempty" gorm:"type:varchar(64)"`
	LastError   string     `json:"last_error,omitempty" gorm:"type:text"`
	QueuedAt    *time.Time `json:"queued_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
