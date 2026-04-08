package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"skills-hub/db"
)

func SyncHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "Only POST method is allowed"})
		return
	}
	if !db.StartTenantSync(sess.CurrentTenantID) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "running", "message": "Sync is already in progress"})
		return
	}

	go func(tenantID int64) {
		slog.Info("manual sync triggered", "tenant_id", tenantID)
		defer db.FinishTenantSync(tenantID)
		if err := db.SyncFromClawHub(tenantID); err != nil {
			slog.Error("Sync failed", "error", err)
		}
	}(sess.CurrentTenantID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "started", "message": "Sync started in background", "startedAt": time.Now().Format(time.RFC3339)})
}

func SyncStatusHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	lastSync, count, status, message, err := db.GetLatestSyncLog(sess.CurrentTenantID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"isSyncing": db.IsSyncing(sess.CurrentTenantID), "lastSync": nil, "count": 0, "status": "", "message": ""})
		return
	}

	var lastSyncValue interface{}
	if lastSync != nil {
		lastSyncValue = lastSync.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"isSyncing": db.IsSyncing(sess.CurrentTenantID), "lastSync": lastSyncValue, "count": count, "status": status, "message": message})
}
