package handlers

import (
	"fmt"
	"net/http"

	"skills-hub/db"
)

func AdminCommentsHandler(w http.ResponseWriter, r *http.Request) {
	comments, err := db.ListAdminComments(200)
	if err != nil {
		http.Error(w, "评论列表加载失败", http.StatusInternalServerError)
		return
	}

	renderAdminPage(w, r, "admin_comments.html", PageData{
		Title:         "评论审核 - Skills Hub",
		AdminSection:  "comments",
		AdminComments: comments,
		Info:          r.URL.Query().Get("info"),
		Error:         r.URL.Query().Get("error"),
	})
}

func AdminCommentDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	commentID, err := parseInt64(r.FormValue("comment_id"))
	if err != nil {
		http.Redirect(w, r, "/admin/comments?error=参数错误", http.StatusSeeOther)
		return
	}

	comment, err := db.GetAdminCommentByID(commentID)
	if err != nil {
		http.Redirect(w, r, "/admin/comments?error=评论加载失败", http.StatusSeeOther)
		return
	}
	if comment == nil {
		http.Redirect(w, r, "/admin/comments?error=评论不存在", http.StatusSeeOther)
		return
	}

	if err := db.DeleteCommentByID(commentID); err != nil {
		http.Redirect(w, r, "/admin/comments?error=评论删除失败", http.StatusSeeOther)
		return
	}

	recordAdminAction(r, "comment.delete", "comment", commentID, fmt.Sprintf("删除了 %s 的评论", comment.UserEmail))
	http.Redirect(w, r, "/admin/comments?info=评论已删除", http.StatusSeeOther)
}
