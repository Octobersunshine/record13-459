package handlers

import (
	"backup-status-api/models"
	"backup-status-api/store"
	"backup-status-api/utils"
	"encoding/json"
	"errors"
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

func (h *BackupHandler) TriggerChecksum(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id := strings.TrimSuffix(path, "/checksum")
	id = strings.TrimSpace(id)

	if id == "" {
		respondError(w, http.StatusBadRequest, "任务ID不能为空")
		return
	}

	checksumType := r.URL.Query().Get("type")
	if checksumType == "" {
		checksumType = "sha256"
	}

	if err := h.StartChecksum(id, checksumType); err != nil {
		if err.Error() == "任务不存在" {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondSuccess(w, map[string]interface{}{
		"id":            id,
		"status":        "running",
		"checksum_type": checksumType,
		"message":       "校验任务已启动，请稍后查询结果",
	})
}

func (h *BackupHandler) StartChecksum(taskID, checksumType string) error {
	task, err := h.store.GetByID(taskID)
	if err != nil {
		return err
	}

	if !task.CanChecksum() {
		return errors.New("任务不满足校验条件：需备份成功且有文件路径")
	}

	go h.processChecksum(task.ID, checksumType)
	return nil
}

func (h *BackupHandler) processChecksum(taskID, checksumType string) {
	task, err := h.store.GetByID(taskID)
	if err != nil {
		return
	}

	task.MarkChecksumRunning()
	h.store.UpdateChecksumResult(task)

	result, err := checksum.Calculate(task.FilePath, checksum.ChecksumType(checksumType))
	if err != nil {
		task.MarkChecksumFailed(err.Error())
		h.store.UpdateChecksumResult(task)
		return
	}

	task.MarkChecksumPassed(checksumType, result.Value)
	h.store.UpdateChecksumResult(task)
}

func (h *BackupHandler) StartRestoreVerify(taskID string) error {
	task, err := h.store.GetByID(taskID)
	if err != nil {
		return err
	}

	if !task.CanRestore() {
		return errors.New("任务不满足恢复验证条件：需备份成功且校验通过")
	}

	go h.processRestoreVerify(task.ID)
	return nil
}

func (h *BackupHandler) TriggerRestoreVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id := strings.TrimSuffix(path, "/restore-verify")
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

	if !task.CanRestore() {
		respondError(w, http.StatusBadRequest, "任务不满足恢复验证条件：需备份成功且校验通过")
		return
	}

	go h.processRestoreVerify(task.ID)

	respondSuccess(w, map[string]interface{}{
		"id":      id,
		"status":  "running",
		"message": "恢复验证任务已启动，请稍后查询结果",
	})
}

func (h *BackupHandler) processRestoreVerify(taskID string) {
	task, err := h.store.GetByID(taskID)
	if err != nil {
		return
	}

	task.MarkRestoreRunning()
	h.store.UpdateRestoreResult(task)

	result, err := checksum.VerifyRestore(task.FilePath, task.BackupSize)
	if err != nil {
		task.MarkRestoreFailed(err.Error())
		h.store.UpdateRestoreResult(task)
		return
	}

	if !result.Success {
		task.MarkRestoreFailed(result.ErrorMessage)
		h.store.UpdateRestoreResult(task)
		return
	}

	task.MarkRestorePassed()
	h.store.UpdateRestoreResult(task)
}

func (h *BackupHandler) GetChecksumResult(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")
	id := strings.TrimSuffix(path, "/checksum")
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
		"id":             task.ID,
		"checksum_status": task.ChecksumStatus,
		"checksum_type":   task.ChecksumType,
		"checksum_value":  task.ChecksumValue,
		"checksum_error":  task.ChecksumError,
		"checksum_at":     task.ChecksumAt,
		"restore_status":  task.RestoreStatus,
		"restore_error":   task.RestoreError,
		"restore_at":      task.RestoreAt,
	})
}

func (h *BackupHandler) BatchTriggerChecksum(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	checksumType := r.URL.Query().Get("type")
	if checksumType == "" {
		checksumType = "sha256"
	}

	pendingTasks := h.store.GetPendingChecksumTasks()
	count := 0

	for _, task := range pendingTasks {
		go h.processChecksum(task.ID, checksumType)
		count++
	}

	respondSuccess(w, map[string]interface{}{
		"triggered_count": count,
		"checksum_type":   checksumType,
		"message":         "已触发所有待校验任务的校验",
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
