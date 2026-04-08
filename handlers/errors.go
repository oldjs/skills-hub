package handlers

import (
	"net/http"
)

// 统一错误页面渲染
func renderErrorPage(w http.ResponseWriter, r *http.Request, code int, title, message string) {
	w.WriteHeader(code)
	RenderTemplate(w, r, "error.html", map[string]interface{}{
		"ErrorCode":    code,
		"ErrorTitle":   title,
		"ErrorMessage": message,
	})
}

func RenderNotFound(w http.ResponseWriter, r *http.Request) {
	renderErrorPage(w, r, 404, "页面不存在", "你访问的页面不存在或已被移除，请检查地址是否正确。")
}

func RenderForbidden(w http.ResponseWriter, r *http.Request) {
	renderErrorPage(w, r, 403, "无权访问", "你没有权限访问此页面。如果你认为这是错误的，请联系管理员。")
}

func RenderServerError(w http.ResponseWriter, r *http.Request) {
	renderErrorPage(w, r, 500, "服务器错误", "服务器遇到了一个意外错误，请稍后重试。如果问题持续存在，请联系管理员。")
}
