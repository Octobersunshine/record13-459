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

	mux.HandleFunc("/api/backup/tasks/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")

		if strings.HasSuffix(path, "/heartbeat") {
			backupHandler.Heartbeat(w, r)
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
	log.Printf("接口列表:")
	log.Printf("  GET    /api/backup/tasks              - 查询备份任务列表")
	log.Printf("  POST   /api/backup/tasks              - 创建备份任务")
	log.Printf("  GET    /api/backup/tasks/{id}         - 查询任务详情")
	log.Printf("  GET    /api/backup/tasks/{id}/status  - 查询任务状态与进度")
	log.Printf("  POST   /api/backup/tasks/{id}/heartbeat - 备份进程上报心跳")
	log.Printf("  POST   /api/backup/tasks/check-timeout - 手动触发超时检测")

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
