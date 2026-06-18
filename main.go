package main

import (
	"backup-status-api/handlers"
	"backup-status-api/models"
	"backup-status-api/store"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	memStore := store.NewMemoryStore()
	backupHandler := handlers.NewBackupHandler(memStore)

	go startTimeoutChecker(memStore)
	go startAutoChecksumService(backupHandler, memStore)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			backupHandler.ListTasks(w, r)
		case http.MethodPost:
			backupHandler.CreateTask(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/backup/tasks/check-timeout", func(w http.ResponseWriter, r *http.Request) {
		backupHandler.CheckTimeout(w, r)
	})

	mux.HandleFunc("/api/backup/tasks/batch-checksum", func(w http.ResponseWriter, r *http.Request) {
		backupHandler.BatchTriggerChecksum(w, r)
	})

	mux.HandleFunc("/api/backup/tasks/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")

		if strings.HasSuffix(path, "/heartbeat") {
			backupHandler.Heartbeat(w, r)
			return
		}

		if strings.HasSuffix(path, "/checksum") {
			if r.Method == http.MethodPost {
				backupHandler.TriggerChecksum(w, r)
			} else {
				backupHandler.GetChecksumResult(w, r)
			}
			return
		}

		if strings.HasSuffix(path, "/restore-verify") {
			backupHandler.TriggerRestoreVerify(w, r)
			return
		}

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if strings.HasSuffix(path, "/status") {
			backupHandler.GetTaskStatus(w, r)
			return
		}

		backupHandler.GetTaskDetail(w, r)
	})

	addr := ":8080"
	log.Printf("服务器启动成功，监听地址: %s", addr)
	log.Printf("默认心跳超时时间: %d 分钟", int(models.DefaultHeartbeatTimeout.Minutes()))
	log.Printf("后台巡检已启动，每 1 分钟自动检测超时任务")
	log.Printf("后台自动校验服务已启动，每 5 分钟自动校验待校验任务")
	log.Printf("接口列表:")
	log.Printf("  GET    /api/backup/tasks                  - 查询备份任务列表(支持 checksum_status, restore_status 筛选)")
	log.Printf("  POST   /api/backup/tasks                  - 创建备份任务")
	log.Printf("  GET    /api/backup/tasks/{id}             - 查询任务详情")
	log.Printf("  GET    /api/backup/tasks/{id}/status      - 查询任务状态与进度")
	log.Printf("  POST   /api/backup/tasks/{id}/heartbeat   - 备份进程上报心跳")
	log.Printf("  POST   /api/backup/tasks/check-timeout    - 手动触发超时检测")
	log.Printf("  GET    /api/backup/tasks/{id}/checksum    - 查询校验结果")
	log.Printf("  POST   /api/backup/tasks/{id}/checksum    - 触发文件校验(支持 type=md5|sha256)")
	log.Printf("  POST   /api/backup/tasks/{id}/restore-verify - 触发恢复验证")
	log.Printf("  POST   /api/backup/tasks/batch-checksum   - 批量触发所有待校验任务")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func startTimeoutChecker(s store.BackupStore) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		count, err := s.CheckTimeoutTasks(models.DefaultHeartbeatTimeout)
		if err != nil {
			log.Printf("[巡检] 检测超时任务失败: %v", err)
			continue
		}
		if count > 0 {
			log.Printf("[巡检] 检测并修正 %d 个超时任务为失败状态", count)
		}
	}
}

func startAutoChecksumService(h *handlers.BackupHandler, s store.BackupStore) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	time.Sleep(10 * time.Second)

	pendingTasks := s.GetPendingChecksumTasks()
	for _, task := range pendingTasks {
		log.Printf("[自动校验] 启动校验任务: %s (%s)", task.ID, task.DatabaseName)
		go func(taskID string) {
			if err := h.StartChecksum(taskID, "sha256"); err != nil {
				log.Printf("[自动校验] 任务 %s 启动失败: %v", taskID, err)
			}
		}(task.ID)
	}

	for range ticker.C {
		pendingTasks := s.GetPendingChecksumTasks()
		if len(pendingTasks) == 0 {
			continue
		}

		log.Printf("[自动校验] 发现 %d 个待校验任务，开始自动校验", len(pendingTasks))

		for _, task := range pendingTasks {
			go func(taskID string) {
				if err := h.StartChecksum(taskID, "sha256"); err != nil {
					log.Printf("[自动校验] 任务 %s 启动失败: %v", taskID, err)
				}
			}(task.ID)
		}
	}
}
