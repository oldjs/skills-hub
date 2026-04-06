package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"skills-hub/db"
)

// POST 提交评分
func RateSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	skillID, err := strconv.ParseInt(r.FormValue("skill_id"), 10, 64)
	if err != nil {
		http.Error(w, "无效的 skill ID", http.StatusBadRequest)
		return
	}

	score, err := strconv.Atoi(r.FormValue("score"))
	if err != nil || score < 1 || score > 5 {
		http.Error(w, "评分必须是 1-5 之间的整数", http.StatusBadRequest)
		return
	}

	if err := db.AddRating(sess.CurrentTenantID, skillID, sess.UserID, score); err != nil {
		http.Error(w, "评分失败", http.StatusInternalServerError)
		return
	}

	// 返回最新的评分统计
	avg, count, err := db.GetSkillRatingStats(sess.CurrentTenantID, skillID)
	if err != nil {
		http.Error(w, "获取评分失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"avgRating": avg,
		"count":     count,
		"userScore": score,
	})
}
