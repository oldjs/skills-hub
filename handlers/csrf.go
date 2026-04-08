package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func getOrCreateCSRFToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie("csrf_token"); err == nil && cookie.Value != "" {
		owner, err := rdb.Get(context.Background(), "csrf:"+cookie.Value).Result()
		if err == nil && owner == csrfOwner(r) {
			return cookie.Value, nil
		}
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := rdb.Set(context.Background(), "csrf:"+token, csrfOwner(r), 24*time.Hour).Err(); err != nil {
		return "", err
	}

	setSecureCookie(w, "csrf_token", token, 86400)
	return token, nil
}

func ValidateCSRFToken(r *http.Request) bool {
	token := r.FormValue("csrf_token")
	if token == "" {
		token = r.Header.Get("X-CSRF-Token")
	}
	if token == "" {
		return false
	}

	cookie, err := r.Cookie("csrf_token")
	if err != nil || cookie.Value == "" || cookie.Value != token {
		return false
	}

	owner, err := rdb.Get(context.Background(), "csrf:"+token).Result()
	if err != nil {
		if err.Error() != "redis: nil" {
			slog.Error("校验 CSRF 失败", "error", err)
		}
		return false
	}
	return owner == csrfOwner(r)
}

func csrfOwner(r *http.Request) string {
	if token := sessionTokenFromRequest(r); token != "" {
		return "session:" + token
	}
	return "guest"
}
