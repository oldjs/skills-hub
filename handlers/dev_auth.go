package handlers

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"skills-hub/db"
)

// devMode 启动时读一次，后续不再变
var devMode = os.Getenv("DEV_MODE") == "true"

// IsDevMode 给外部（main.go）判断用
func IsDevMode() bool { return devMode }

// DevAutoLogin 全局中间件：DEV_MODE 下自动为匿名请求注入 dev 用户 session
// 只在 DEV_MODE=true 时通过 main.go 注册到 handler 链，生产环境完全不存在
func DevAutoLogin(next http.Handler) http.Handler {
	// 启动时打个醒目日志
	log.Println("[DEV_MODE] auto-login middleware active -- DO NOT use in production")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 已经有有效 session 就不管了
		if sess := getSession(r); sess != nil {
			next.ServeHTTP(w, r)
			return
		}

		// 静态资源、健康检查这些不需要 session
		if shouldSkipDevLogin(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 找 dev 用户，拿不到就跳过（可能 seed 没跑成功）
		user, err := db.GetUserByEmail("dev@localhost")
		if err != nil || user == nil {
			slog.Warn("[DEV_MODE] dev user not found, skipping auto-login")
			next.ServeHTTP(w, r)
			return
		}

		// 给 dev 用户找个可用租户
		tenant, err := ensureUserTenant(user)
		if err != nil || tenant == nil {
			slog.Warn("[DEV_MODE] no active tenant for dev user, skipping auto-login")
			next.ServeHTTP(w, r)
			return
		}

		// 创建真实 session 写到 Redis + cookie
		if err := createUserSession(w, user, tenant); err != nil {
			slog.Error("[DEV_MODE] failed to create dev session", "error", err)
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// 静态资源和无状态端点不需要自动登录
func shouldSkipDevLogin(path string) bool {
	switch {
	case len(path) > 8 && path[:8] == "/static/":
		return true
	case path == "/healthz":
		return true
	case path == "/captcha":
		return true
	case path == "/robots.txt":
		return true
	case path == "/sitemap.xml":
		return true
	case path == "/favicon.ico":
		return true
	}
	return false
}
