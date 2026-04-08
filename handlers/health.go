package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"skills-hub/db"
)

// GET /healthz — 健康检查，检测 DB + Redis 可达性
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	status := "ok"
	dbOk := checkDB()
	redisOk := checkRedis()

	if !dbOk || !redisOk {
		status = "degraded"
	}

	code := http.StatusOK
	if !dbOk && !redisOk {
		status = "down"
		code = http.StatusServiceUnavailable
	}

	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"db":     boolToStatus(dbOk),
		"redis":  boolToStatus(redisOk),
	})
}

func checkDB() bool {
	return db.GetDB().Ping() == nil
}

func checkRedis() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err() == nil
}

func boolToStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}
