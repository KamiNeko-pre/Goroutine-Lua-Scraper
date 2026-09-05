package task

import "time"

// TaskOutbox records whether one durable CrawlTask has been published to the
// Redis Stream. A nil PublishedAt means the publisher must still deliver it.
type TaskOutbox struct {
	ID          uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID      uint64     `json:"task_id" gorm:"not null;uniqueIndex"`
	MessageID   string     `json:"message_id,omitempty" gorm:"type:varchar(128)"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
