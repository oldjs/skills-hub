package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"skills-hub/db"
)

// POST /api/comment/vote — 评论投票
func CommentVoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	commentID, err := strconv.ParseInt(r.FormValue("comment_id"), 10, 64)
	if err != nil || commentID <= 0 {
		http.Error(w, "参数错误", http.StatusBadRequest)
		return
	}

	vote, err := strconv.Atoi(r.FormValue("vote"))
	if err != nil || (vote != 1 && vote != -1) {
		http.Error(w, "vote 必须是 1 或 -1", http.StatusBadRequest)
		return
	}

	upvotes, downvotes, err := db.VoteComment(commentID, sess.UserID, vote)
	if err != nil {
		http.Error(w, "投票失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"upvotes":   upvotes,
		"downvotes": downvotes,
	})
}
