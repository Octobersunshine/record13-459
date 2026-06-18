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

type ChecksumStatus string

const (
	ChecksumPending  ChecksumStatus = "pending"
	ChecksumRunning ChecksumStatus = "running"
	ChecksumPassed  ChecksumStatus = "passed"
	ChecksumFailed  ChecksumStatus = "failed"
	ChecksumSkipped ChecksumStatus = "skipped"
)

type RestoreStatus string

const (
	RestorePending  RestoreStatus = "pending"
	RestoreRunning RestoreStatus = "running"
	RestorePassed  RestoreStatus = "passed"
	RestoreFailed  RestoreStatus = "failed"
	RestoreSkipped RestoreStatus = "skipped"
)

type BackupTask struct {
	ID                string         `json:"id"`
	DatabaseName      string       `json:"database_name"`
	Status            BackupStatus   `json:"status"`
	Progress          int          `json:"progress"`
	TotalSize         int64        `json:"total_size"`
	BackupSize        int64        `json:"backup_size"`
	StartTime         *time.Time   `json:"start_time"`
	EndTime           *time.Time   `json:"end_time"`
	ErrorMessage      string       `json:"error_message,omitempty"`
	BackupType        string       `json:"backup_type"`
	FilePath          string       `json:"file_path,omitempty"`
	LastHeartbeatAt   *time.Time   `json:"last_heartbeat_at,omitempty"`
	ChecksumStatus    ChecksumStatus `json:"checksum_status"`
	ChecksumType      string       `json:"checksum_type,omitempty"`
	ChecksumValue     string       `json:"checksum_value,omitempty"`
	ChecksumError     string       `json:"checksum_error,omitempty"`
	ChecksumAt        *time.Time   `json:"checksum_at,omitempty"`
	RestoreStatus     RestoreStatus  `json:"restore_status"`
	RestoreError      string       `json:"restore_error,omitempty"`
	RestoreAt         *time.Time   `json:"restore_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
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

func (t *BackupTask) CanChecksum() bool {
	return t.Status == StatusSuccess && t.FilePath != "" &&
		t.ChecksumStatus != ChecksumRunning
}

func (t *BackupTask) CanRestore() bool {
	return t.Status == StatusSuccess && t.FilePath != "" &&
		t.ChecksumStatus == ChecksumPassed &&
		t.RestoreStatus != RestoreRunning
}

func (t *BackupTask) MarkChecksumRunning() {
	now := time.Now()
	t.ChecksumStatus = ChecksumRunning
	t.UpdatedAt = now
}

func (t *BackupTask) MarkChecksumPassed(checksumType, checksumValue string) {
	now := time.Now()
	t.ChecksumStatus = ChecksumPassed
	t.ChecksumType = checksumType
	t.ChecksumValue = checksumValue
	t.ChecksumError = ""
	t.ChecksumAt = &now
	t.UpdatedAt = now
}

func (t *BackupTask) MarkChecksumFailed(errMsg string) {
	now := time.Now()
	t.ChecksumStatus = ChecksumFailed
	t.ChecksumError = errMsg
	t.ChecksumAt = &now
	t.UpdatedAt = now
}

func (t *BackupTask) MarkRestoreRunning() {
	now := time.Now()
	t.RestoreStatus = RestoreRunning
	t.UpdatedAt = now
}

func (t *BackupTask) MarkRestorePassed() {
	now := time.Now()
	t.RestoreStatus = RestorePassed
	t.RestoreError = ""
	t.RestoreAt = &now
	t.UpdatedAt = now
}

func (t *BackupTask) MarkRestoreFailed(errMsg string) {
	now := time.Now()
	t.RestoreStatus = RestoreFailed
	t.RestoreError = errMsg
	t.RestoreAt = &now
	t.UpdatedAt = now
}

type TaskListResponse struct {
	Total int64        `json:"total"`
	Tasks []BackupTask `json:"tasks"`
}

type TaskQuery struct {
	Status         *BackupStatus   `json:"status,omitempty"`
	ChecksumStatus *ChecksumStatus `json:"checksum_status,omitempty"`
	RestoreStatus  *RestoreStatus  `json:"restore_status,omitempty"`
	Keyword        string          `json:"keyword,omitempty"`
	Page           int             `json:"page"`
	Size           int             `json:"size"`
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
