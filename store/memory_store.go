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
	task1 := &models.BackupTask{
		ID:           uuid.New().String(),
		DatabaseName: "user_db",
		Status:       models.StatusSuccess,
		Progress:     100,
		TotalSize:    1024 * 1024 * 500,
		BackupSize:   1024 * 1024 * 500,
		StartTime:    &start1,
		EndTime:      &end1,
		BackupType:   "full",
		FilePath:     "/backups/user_db_full_20240101.sql",
		CreatedAt:    start1,
		UpdatedAt:    end1,
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
	task5 := &models.BackupTask{
		ID:           uuid.New().String(),
		DatabaseName: "product_db",
		Status:       models.StatusSuccess,
		Progress:     100,
		TotalSize:    1024 * 1024 * 256,
		BackupSize:   1024 * 1024 * 256,
		StartTime:    &start5,
		EndTime:      &end5,
		BackupType:   "incremental",
		FilePath:     "/backups/product_db_inc_20240101.sql",
		CreatedAt:    start5,
		UpdatedAt:    end5,
	}

	s.tasks[task1.ID] = task1
	s.tasks[task2.ID] = task2
	s.tasks[task3.ID] = task3
	s.tasks[task4.ID] = task4
	s.tasks[task5.ID] = task5
}

func (s *MemoryStore) GetByID(id string) (*models.BackupTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, errors.New("任务不存在")
	}

	result := *task
	return &result, nil
}

func (s *MemoryStore) List(query *models.TaskQuery) (*models.TaskListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*models.BackupTask
	for _, task := range s.tasks {
		if query.Status != nil && task.Status != *query.Status {
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
