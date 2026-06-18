package main

import (
	"backup-status-api/handlers"
	"backup-status-api/store"
	"log"
	"net/http"
	"strings"
)

func main() {
	memStore := store.NewMemoryStore()
	backupHandler := handlers.NewBackupHandler(memStore)

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

	mux.HandleFunc("/api/backup/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/backup/tasks/")

		if strings.HasSuffix(path, "/status") {
			backupHandler.GetTaskStatus(w, r)
			return
		}

		backupHandler.GetTaskDetail(w, r)
	})

	addr := ":8080"
	log.Printf("服务器启动成功，监听地址: %s", addr)
	log.Printf("接口列表:")
	log.Printf("  GET  /api/backup/tasks          - 查询备份任务列表")
	log.Printf("  GET  /api/backup/tasks/{id}     - 查询任务详情")
	log.Printf("  GET  /api/backup/tasks/{id}/status - 查询任务状态与进度")
	log.Printf("  POST /api/backup/tasks          - 创建备份任务")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
