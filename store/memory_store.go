package store

import (
	"backup-status-api/models"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type BackupStore interface {
	GetByID(id string) (*models.BackupTask, error)
	List(query *models.TaskQuery) (*models.TaskListResponse, error)
	Create(task *models.BackupTask) error
	Update(task *models.BackupTask) error
	UpdateHeartbeat(id string) error
	CheckTimeoutTasks(timeout time.Duration) (int64, error)
	GetPendingChecksumTasks() []*models.BackupTask
	UpdateChecksumResult(task *models.BackupTask) error
	UpdateRestoreResult(task *models.BackupTask) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*models.BackupTask
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		tasks: make(map[string]*models.BackupTask),
	}
	store.initMockData()
	return store
}

func (s *MemoryStore) initMockData() {
	now := time.Now()

	start1 := now.Add(-2 * time.Hour)
	end1 := now.Add(-1 * time.Hour)
	checksumAt1 := end1
	task1 := &models.BackupTask{
		ID:             uuid.New().String(),
		DatabaseName:   "user_db",
		Status:         models.StatusSuccess,
		Progress:       100,
		TotalSize:      1024 * 1024 * 500,
		BackupSize:     1024 * 1024 * 500,
		StartTime:      &start1,
		EndTime:        &end1,
		BackupType:     "full",
		FilePath:       "/backups/user_db_full_20240101.sql",
		ChecksumStatus: models.ChecksumPassed,
		ChecksumType:   "sha256",
		ChecksumValue:  "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
		ChecksumAt:     &checksumAt1,
		RestoreStatus:  models.RestorePassed,
		RestoreAt:      &checksumAt1,
		CreatedAt:      start1,
		UpdatedAt:      end1,
	}

	start2 := now.Add(-30 * time.Minute)
	task2 := &models.BackupTask{
		ID:           uuid.New().String(),
		DatabaseName: "order_db",
		Status:       models.StatusRunning,
		Progress:     65,
		TotalSize:    1024 * 1024 * 1024,
		BackupSize:   1024 * 1024 * 665,
		StartTime:    &start2,
		BackupType:   "incremental",
		CreatedAt:    start2,
		UpdatedAt:    now,
	}

	start3 := now.Add(-3 * time.Hour)
	end3 := now.Add(-2 * time.Hour)
	task3 := &models.BackupTask{
		ID:           uuid.New().String(),
		DatabaseName: "log_db",
		Status:       models.StatusFailed,
		Progress:     45,
		TotalSize:    1024 * 1024 * 2048,
		BackupSize:   1024 * 1024 * 922,
		StartTime:    &start3,
		EndTime:      &end3,
		ErrorMessage: "磁盘空间不足，备份中断",
		BackupType:   "full",
		CreatedAt:    start3,
		UpdatedAt:    end3,
	}

	task4 := &models.BackupTask{
		ID:           uuid.New().String(),
		DatabaseName: "analytics_db",
		Status:       models.StatusPending,
		Progress:     0,
		TotalSize:    1024 * 1024 * 512,
		BackupType:   "full",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	start5 := now.Add(-5 * time.Hour)
	end5 := now.Add(-4 * time.Hour)
	checksumAt5 := end5
	task5 := &models.BackupTask{
		ID:             uuid.New().String(),
		DatabaseName:   "product_db",
		Status:         models.StatusSuccess,
		Progress:       100,
		TotalSize:      1024 * 1024 * 256,
		BackupSize:     1024 * 1024 * 256,
		StartTime:      &start5,
		EndTime:        &end5,
		BackupType:     "incremental",
		FilePath:       "/backups/product_db_inc_20240101.sql",
		ChecksumStatus: models.ChecksumPending,
		RestoreStatus:  models.RestorePending,
		CreatedAt:      start5,
		UpdatedAt:      end5,
	}

	s.tasks[task1.ID] = task1
	s.tasks[task2.ID] = task2
	s.tasks[task3.ID] = task3
	s.tasks[task4.ID] = task4
	s.tasks[task5.ID] = task5
}

func (s *MemoryStore) GetByID(id string) (*models.BackupTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, errors.New("任务不存在")
	}

	if task.IsTimeout(models.DefaultHeartbeatTimeout) {
		task.MarkAsFailed("备份进程心跳超时，疑似异常崩溃")
	}

	result := *task
	return &result, nil
}

func (s *MemoryStore) UpdateHeartbeat(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return errors.New("任务不存在")
	}

	if task.Status != models.StatusRunning {
		return errors.New("任务不在运行中，无法更新心跳")
	}

	now := time.Now()
	task.LastHeartbeatAt = &now
	task.UpdatedAt = now
	return nil
}

func (s *MemoryStore) CheckTimeoutTasks(timeout time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	for _, task := range s.tasks {
		if task.IsTimeout(timeout) {
			task.MarkAsFailed("备份进程心跳超时，疑似异常崩溃")
			count++
		}
	}
	return count, nil
}

func (s *MemoryStore) List(query *models.TaskQuery) (*models.TaskListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		if task.IsTimeout(models.DefaultHeartbeatTimeout) {
			task.MarkAsFailed("备份进程心跳超时，疑似异常崩溃")
		}
	}

	var filtered []*models.BackupTask
	for _, task := range s.tasks {
		if query.Status != nil && task.Status != *query.Status {
			continue
		}
		if query.ChecksumStatus != nil && task.ChecksumStatus != *query.ChecksumStatus {
			continue
		}
		if query.RestoreStatus != nil && task.RestoreStatus != *query.RestoreStatus {
			continue
		}
		if query.Keyword != "" && !strings.Contains(strings.ToLower(task.DatabaseName), strings.ToLower(query.Keyword)) {
			continue
		}
		filtered = append(filtered, task)
	}

	total := int64(len(filtered))

	page := query.GetPage()
	size := query.GetSize()
	start := (page - 1) * size
	end := start + size

	if start >= len(filtered) {
		return &models.TaskListResponse{
			Total: total,
			Tasks: []models.BackupTask{},
		}, nil
	}

	if end > len(filtered) {
		end = len(filtered)
	}

	resultTasks := make([]models.BackupTask, 0, end-start)
	for _, task := range filtered[start:end] {
		resultTasks = append(resultTasks, *task)
	}

	return &models.TaskListResponse{
		Total: total,
		Tasks: resultTasks,
	}, nil
}

func (s *MemoryStore) Create(task *models.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryStore) Update(task *models.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.tasks[task.ID]
	if !exists {
		return errors.New("任务不存在")
	}

	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
	return nil
}

func (s *MemoryStore) GetPendingChecksumTasks() []*models.BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []*models.BackupTask
	for _, task := range s.tasks {
		if task.CanChecksum() && task.ChecksumStatus == models.ChecksumPending {
			pending = append(pending, task)
		}
	}
	return pending
}

func (s *MemoryStore) UpdateChecksumResult(task *models.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tasks[task.ID]
	if !exists {
		return errors.New("任务不存在")
	}

	existing.ChecksumStatus = task.ChecksumStatus
	existing.ChecksumType = task.ChecksumType
	existing.ChecksumValue = task.ChecksumValue
	existing.ChecksumError = task.ChecksumError
	existing.ChecksumAt = task.ChecksumAt
	existing.UpdatedAt = time.Now()

	return nil
}

func (s *MemoryStore) UpdateRestoreResult(task *models.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.tasks[task.ID]
	if !exists {
		return errors.New("任务不存在")
	}

	existing.RestoreStatus = task.RestoreStatus
	existing.RestoreError = task.RestoreError
	existing.RestoreAt = task.RestoreAt
	existing.UpdatedAt = time.Now()

	return nil
}
