package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

func AccountHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	apiKeys, err := db.ListUserAPIKeys(sess.UserID)
	if err != nil {
		http.Error(w, "API Key 列表加载失败", http.StatusInternalServerError)
		return
	}
	if apiKeys == nil {
		apiKeys = []models.APIKey{}
	}

	RenderTemplate(w, r, "account.html", PageData{
		Title:       "个人中心 - Skills Hub",
		CurrentPage: "account",
		APIKeys:     apiKeys,
		Info:        r.URL.Query().Get("info"),
		Error:       r.URL.Query().Get("error"),
	})
}

func AccountCreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = "默认密钥"
	}
	rawKey, err := generateAPIKey()
	if err != nil {
		http.Error(w, "生成 API Key 失败", http.StatusInternalServerError)
		return
	}
	prefix := rawKey
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	_, err = db.CreateAPIKey(sess.UserID, db.HashAPIKey(rawKey), prefix, name)
	if err != nil {
		http.Error(w, "保存 API Key 失败", http.StatusInternalServerError)
		return
	}

	apiKeys, err := db.ListUserAPIKeys(sess.UserID)
	if err != nil {
		http.Error(w, "API Key 列表加载失败", http.StatusInternalServerError)
		return
	}
	if apiKeys == nil {
		apiKeys = []models.APIKey{}
	}

	RenderTemplate(w, r, "account.html", PageData{
		Title:           "个人中心 - Skills Hub",
		CurrentPage:     "account",
		APIKeys:         apiKeys,
		GeneratedAPIKey: rawKey,
		Info:            "新的 API Key 已生成，请现在复制保存，离开页面后将不再展示完整值",
	})
}

func AccountRevokeAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	sess := GetCurrentSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	keyID, err := parseInt64(r.FormValue("key_id"))
	if err != nil {
		http.Redirect(w, r, "/account?error=参数错误", http.StatusSeeOther)
		return
	}
	if err := db.RevokeAPIKey(keyID, sess.UserID); err != nil {
		http.Redirect(w, r, "/account?error=撤销 API Key 失败", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/account?info=API Key 已撤销", http.StatusSeeOther)
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "shk_" + hex.EncodeToString(buf), nil
}
