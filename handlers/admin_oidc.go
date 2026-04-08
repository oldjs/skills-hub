package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

	"skills-hub/db"
	"skills-hub/models"
)

// GET /admin/oauth-clients 管理 OIDC 客户端
func AdminOAuthClientsHandler(w http.ResponseWriter, r *http.Request) {
	clients, err := db.ListOAuthClients()
	if err != nil {
		log.Printf("list oauth clients failed: %v", err)
		clients = []models.OAuthClient{}
	}

	renderData := map[string]interface{}{
		"Title":        "OIDC 客户端管理 - Skills Hub",
		"AdminSection": "oauth_clients",
		"CurrentPage":  "admin",
		"OAuthClients": clients,
		"Info":         r.URL.Query().Get("info"),
		"Error":        r.URL.Query().Get("error"),
	}
	// 刚创建的客户端密钥（只展示一次）
	if newID := r.URL.Query().Get("new_client_id"); newID != "" {
		renderData["NewClientID"] = newID
		renderData["NewClientSecret"] = r.URL.Query().Get("new_client_secret")
	}
	RenderTemplate(w, r, "admin_oauth_clients.html", renderData)
}

// POST /admin/oauth-clients/create
func AdminOAuthClientCreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	redirectURIsStr := strings.TrimSpace(r.FormValue("redirect_uris"))

	if name == "" {
		http.Redirect(w, r, "/admin/oauth-clients?error=名称不能为空", http.StatusSeeOther)
		return
	}

	// 解析 redirect_uris（每行一个）
	var redirectURIs []string
	for _, uri := range strings.Split(redirectURIsStr, "\n") {
		uri = strings.TrimSpace(uri)
		if uri != "" {
			redirectURIs = append(redirectURIs, uri)
		}
	}
	if len(redirectURIs) == 0 {
		http.Redirect(w, r, "/admin/oauth-clients?error=至少填写一个回调地址", http.StatusSeeOther)
		return
	}

	// 生成 client_id 和 client_secret
	clientID, err := generateShortID(16)
	if err != nil {
		http.Redirect(w, r, "/admin/oauth-clients?error=生成 Client ID 失败", http.StatusSeeOther)
		return
	}
	clientSecret, err := generateShortID(32)
	if err != nil {
		http.Redirect(w, r, "/admin/oauth-clients?error=生成 Client Secret 失败", http.StatusSeeOther)
		return
	}

	if err := db.CreateOAuthClient(clientID, name, clientSecret, redirectURIs); err != nil {
		log.Printf("create oauth client failed: %v", err)
		http.Redirect(w, r, "/admin/oauth-clients?error=创建失败", http.StatusSeeOther)
		return
	}

	// 跳回列表页，带上刚生成的 secret（只展示一次）
	http.Redirect(w, r, "/admin/oauth-clients?info=客户端已创建&new_client_id="+clientID+"&new_client_secret="+clientSecret, http.StatusSeeOther)
}

// POST /admin/oauth-clients/delete
func AdminOAuthClientDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	clientID := strings.TrimSpace(r.FormValue("client_id"))
	if clientID == "" {
		http.Redirect(w, r, "/admin/oauth-clients", http.StatusSeeOther)
		return
	}

	if err := db.DeleteOAuthClient(clientID); err != nil {
		log.Printf("delete oauth client failed: %v", err)
	}

	http.Redirect(w, r, "/admin/oauth-clients?info=客户端已删除", http.StatusSeeOther)
}

func generateShortID(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
