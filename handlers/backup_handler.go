package handlers

import (
	"backup-status-api/models"
	"backup-status-api/store"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type BackupHandler struct {
	store store.BackupStore
}

func NewBackupHandler(s store.BackupStore) *BackupHandler {
	return &BackupHandler{store: s}
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *BackupHandler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id = strings.TrimSuffix(id, "/status")
	id = strings.TrimSpace(id)

	if id == "" {
		respondError(w, http.StatusBadRequest, "任务ID不能为空")
		return
	}

	task, err := h.store.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondSuccess(w, map[string]interface{}{
		"id":         task.ID,
		"status":     task.Status,
		"progress":   task.Progress,
		"start_time": task.StartTime,
		"end_time":   task.EndTime,
	})
}

func (h *BackupHandler) GetTaskDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id = strings.TrimSpace(id)

	if id == "" {
		respondError(w, http.StatusBadRequest, "任务ID不能为空")
		return
	}

	task, err := h.store.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondSuccess(w, task)
}

func (h *BackupHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := &models.TaskQuery{}

	statusStr := r.URL.Query().Get("status")
	if statusStr != "" {
		status := models.BackupStatus(statusStr)
		query.Status = &status
	}

	query.Keyword = r.URL.Query().Get("keyword")

	pageStr := r.URL.Query().Get("page")
	if pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err == nil {
			query.Page = page
		}
	}

	sizeStr := r.URL.Query().Get("size")
	if sizeStr != "" {
		size, err := strconv.Atoi(sizeStr)
		if err == nil {
			query.Size = size
		}
	}

	result, err := h.store.List(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondSuccess(w, result)
}

func (h *BackupHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var task models.BackupTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondError(w, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	if task.DatabaseName == "" {
		respondError(w, http.StatusBadRequest, "数据库名称不能为空")
		return
	}

	task.Status = models.StatusPending
	task.Progress = 0

	if err := h.store.Create(&task); err != nil {
		respondError(w, http.StatusInternalServerError, "创建任务失败: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	respondSuccess(w, task)
}

func respondSuccess(w http.ResponseWriter, data interface{}) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *BackupHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id := strings.TrimSuffix(path, "/heartbeat")
	id = strings.TrimSpace(id)

	if id == "" {
		respondError(w, http.StatusBadRequest, "任务ID不能为空")
		return
	}

	if err := h.store.UpdateHeartbeat(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondSuccess(w, map[string]interface{}{
		"id":            id,
		"status":        "running",
		"heartbeat_at":  time.Now(),
	})
}

func (h *BackupHandler) CheckTimeout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	timeout := models.DefaultHeartbeatTimeout
	timeoutStr := r.URL.Query().Get("timeout_minutes")
	if timeoutStr != "" {
		minutes, err := strconv.Atoi(timeoutStr)
		if err == nil && minutes > 0 {
			timeout = time.Duration(minutes) * time.Minute
		}
	}

	count, err := h.store.CheckTimeoutTasks(timeout)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondSuccess(w, map[string]interface{}{
		"timeout_minutes": int(timeout.Minutes()),
		"corrected_count": count,
		"message":         "巡检完成",
	})
}

func respondError(w http.ResponseWriter, code int, message string) {
	resp := Response{
		Code:    code,
		Message: message,
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}
