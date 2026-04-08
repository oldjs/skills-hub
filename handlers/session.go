package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 客户端，auth 初始化时赋值
var rdb *redis.Client

const sessionTTL = 24 * time.Hour

type sessionData struct {
	UserID          int64  `json:"user_id"`
	Email           string `json:"email"`
	DisplayName     string `json:"display_name"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	IsSubAdmin      bool   `json:"is_sub_admin"`
	CurrentTenantID int64  `json:"current_tenant_id"`
	TenantName      string `json:"tenant_name"`
	TenantSlug      string `json:"tenant_slug"`
	TenantRole      string `json:"tenant_role"`
}

func InitAuth() error {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		PoolSize: 20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err()
}

func GetRedisClient() *redis.Client { return rdb }

func CloseAuth() {
	if rdb != nil {
		_ = rdb.Close()
	}
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sessionKey(token string) string {
	return "session:" + token
}

func setSession(token string, sess sessionData) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return rdb.Set(context.Background(), sessionKey(token), data, sessionTTL).Err()
}

func getSession(r *http.Request) *sessionData {
	cookie, err := r.Cookie("session")
	if err != nil || cookie.Value == "" {
		return nil
	}

	data, err := rdb.Get(context.Background(), sessionKey(cookie.Value)).Bytes()
	if err != nil {
		if err != redis.Nil {
			slog.Error("读取会话失败", "error", err)
		}
		return nil
	}

	var sess sessionData
	if err := json.Unmarshal(data, &sess); err != nil {
		slog.Error("解析会话失败", "error", err)
		return nil
	}

	return &sess
}

func deleteSession(token string) {
	if err := rdb.Del(context.Background(), sessionKey(token)).Err(); err != nil && err != redis.Nil {
		slog.Error("删除会话失败", "error", err)
	}
}

func isCookieSecure() bool {
	return os.Getenv("COOKIE_SECURE") == "true"
}

func setSecureCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	setSecureCookie(w, "session", token, int(sessionTTL.Seconds()))
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isCookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// 对外暴露的 session 查询
func IsLoggedIn(r *http.Request) bool   { return getSession(r) != nil }
func GetCurrentSession(r *http.Request) *sessionData { return getSession(r) }

func IsPlatformAdmin(r *http.Request) bool {
	sess := getSession(r)
	return sess != nil && sess.IsPlatformAdmin
}

func IsSubAdmin(r *http.Request) bool {
	sess := getSession(r)
	return sess != nil && sess.IsSubAdmin
}

func IsAdmin(r *http.Request) bool {
	sess := getSession(r)
	return sess != nil && (sess.IsPlatformAdmin || sess.IsSubAdmin)
}
