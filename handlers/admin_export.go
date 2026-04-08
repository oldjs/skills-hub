package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"skills-hub/db"
)

// GET /admin/logs/export — 导出操作日志为 CSV
func AdminLogsExportHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := db.ListAdminActionLogs(1000)
	if err != nil {
		http.Error(w, "日志加载失败", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("admin-logs-%s.csv", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	// BOM 让 Excel 正确识别 UTF-8
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"ID", "管理员", "邮箱", "操作", "目标类型", "目标ID", "详情", "时间"})

	for _, log := range logs {
		_ = writer.Write([]string{
			fmt.Sprintf("%d", log.ID),
			log.AdminName,
			log.AdminEmail,
			log.Action,
			log.TargetType,
			fmt.Sprintf("%d", log.TargetID),
			log.Details,
			log.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}
