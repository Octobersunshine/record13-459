package models

import "time"

type BackupStatus string

const (
	StatusPending   BackupStatus = "pending"
	StatusRunning BackupStatus = "running"
	StatusSuccess BackupStatus = "success"
	StatusFailed  BackupStatus = "failed"
)

const DefaultHeartbeatTimeout = 5 * time.Minute

type BackupTask struct {
	ID              string       `json:"id"`
	DatabaseName    string     `json:"database_name"`
	Status          BackupStatus `json:"status"`
	Progress        int        `json:"progress"`
	TotalSize       int64      `json:"total_size"`
	BackupSize      int64      `json:"backup_size"`
	StartTime       *time.Time `json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	BackupType      string     `json:"backup_type"`
	FilePath        string     `json:"file_path,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (t *BackupTask) IsTimeout(timeout time.Duration) bool {
	if t.Status != StatusRunning {
		return false
	}
	if t.LastHeartbeatAt == nil {
		if t.StartTime == nil {
			return false
		}
		return time.Since(*t.StartTime) > timeout
	}
	return time.Since(*t.LastHeartbeatAt) > timeout
}

func (t *BackupTask) MarkAsFailed(reason string) {
	now := time.Now()
	t.Status = StatusFailed
	t.EndTime = &now
	t.ErrorMessage = reason
	t.UpdatedAt = now
}

type TaskListResponse struct {
	Total int64        `json:"total"`
	Tasks []BackupTask `json:"tasks"`
}

type TaskQuery struct {
	Status  *BackupStatus `json:"status,omitempty"`
	Keyword string        `json:"keyword,omitempty"`
	Page    int           `json:"page"`
	Size    int           `json:"size"`
}

func (q *TaskQuery) GetPage() int {
	if q.Page <= 0 {
		return 1
	}
	return q.Page
}

func (q *TaskQuery) GetSize() int {
	if q.Size <= 0 {
		return 10
	}
	return q.Size
}
