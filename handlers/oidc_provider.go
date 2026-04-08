package handlers

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"skills-hub/db"
	"skills-hub/models"
)

func oidcIssuer() string {
	iss := os.Getenv("OIDC_ISSUER")
	if iss == "" {
		iss = "http://localhost:8080"
	}
	return strings.TrimRight(iss, "/")
}

// GET /.well-known/openid-configuration
func OIDCDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	iss := oidcIssuer()
	discovery := map[string]interface{}{
		"issuer":                 iss,
		"authorization_endpoint": iss + "/oauth/authorize",
		"token_endpoint":         iss + "/oauth/token",
		"userinfo_endpoint":      iss + "/oauth/userinfo",
		"jwks_uri":               iss + "/oauth/jwks",
		"response_types_supported": []string{"code"},
		"subject_types_supported":  []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":        []string{"openid", "profile", "email"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"claims_supported":        []string{"sub", "email", "name", "iss", "aud", "exp", "iat"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(discovery)
}

// GET /oauth/jwks
func OIDCJWKSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(buildJWKS())
}

// GET /oauth/authorize
func OIDCAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	// 必须已登录
	sess := GetCurrentSession(r)
	if sess == nil {
		// 把当前 authorize URL 存起来，登录后跳回
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.String()), http.StatusSeeOther)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	nonce := r.URL.Query().Get("nonce")

	// 校验基本参数
	if responseType != "code" {
		http.Error(w, "unsupported_response_type", http.StatusBadRequest)
		return
	}

	// 校验 client
	client, err := db.GetOAuthClient(clientID)
	if err != nil || client == nil {
		http.Error(w, "invalid_client", http.StatusBadRequest)
		return
	}

	// 校验 redirect_uri
	if !isValidRedirectURI(client, redirectURI) {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	// 处理用户同意（GET 展示确认页，POST 处理同意）
	if r.Method == http.MethodPost {
		if !ValidateCSRFToken(r) {
			http.Error(w, "无效的请求", http.StatusForbidden)
			return
		}
		handleAuthorizeConsent(w, r, sess, client, redirectURI, scope, state, nonce)
		return
	}

	// GET: 展示授权确认页
	data := PageData{
		Title:       "授权确认 - Skills Hub",
		CurrentPage: "oauth_consent",
	}
	RenderTemplate(w, r, "oauth_consent.html", map[string]interface{}{
		"Title":       data.Title,
		"ClientName":  client.Name,
		"ClientID":    client.ID,
		"RedirectURI": redirectURI,
		"Scope":       scope,
		"State":       state,
		"Nonce":       nonce,
	})
}

func handleAuthorizeConsent(w http.ResponseWriter, r *http.Request, sess *sessionData, client *models.OAuthClient, redirectURI, scope, state, nonce string) {
	// 生成授权码
	code, err := generateToken()
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	// 授权码存 Redis，5 分钟有效
	codeData := map[string]interface{}{
		"user_id":      sess.UserID,
		"client_id":    client.ID,
		"redirect_uri": redirectURI,
		"scope":        scope,
		"nonce":        nonce,
	}
	codeJSON, _ := json.Marshal(codeData)
	codeKey := "oidc_code:" + code
	if err := rdb.Set(context.Background(), codeKey, codeJSON, 5*time.Minute).Err(); err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	// 跳回客户端
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	target := redirectURI + sep + "code=" + url.QueryEscape(code)
	if state != "" {
		target += "&state=" + url.QueryEscape(state)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// POST /oauth/token
func OIDCTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 解析 client 认证（支持 body 和 basic auth）
	clientID, clientSecret := extractClientCredentials(r)
	if clientID == "" || clientSecret == "" {
		writeOIDCError(w, "invalid_client", "missing client credentials", http.StatusUnauthorized)
		return
	}

	// 校验 client
	client, err := db.VerifyOAuthClientSecret(clientID, clientSecret)
	if err != nil || client == nil {
		writeOIDCError(w, "invalid_client", "client authentication failed", http.StatusUnauthorized)
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		writeOIDCError(w, "unsupported_grant_type", "", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")

	// 从 Redis 取授权码
	codeKey := "oidc_code:" + code
	codeJSON, err := rdb.GetDel(context.Background(), codeKey).Bytes()
	if err != nil {
		writeOIDCError(w, "invalid_grant", "authorization code expired or invalid", http.StatusBadRequest)
		return
	}

	var codeData map[string]interface{}
	if err := json.Unmarshal(codeJSON, &codeData); err != nil {
		writeOIDCError(w, "invalid_grant", "", http.StatusBadRequest)
		return
	}

	// 校验 client_id 和 redirect_uri 一致
	if codeData["client_id"] != clientID {
		writeOIDCError(w, "invalid_grant", "client_id mismatch", http.StatusBadRequest)
		return
	}
	if codeData["redirect_uri"] != redirectURI {
		writeOIDCError(w, "invalid_grant", "redirect_uri mismatch", http.StatusBadRequest)
		return
	}

	userID := int64(codeData["user_id"].(float64))
	scope := fmt.Sprintf("%v", codeData["scope"])
	nonce := fmt.Sprintf("%v", codeData["nonce"])

	// 拿用户信息
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		writeOIDCError(w, "invalid_grant", "user not found", http.StatusBadRequest)
		return
	}

	// 生成 access_token，存 Redis
	accessToken, err := generateToken()
	if err != nil {
		writeOIDCError(w, "server_error", "", http.StatusInternalServerError)
		return
	}
	tokenData := map[string]interface{}{
		"user_id":   userID,
		"client_id": clientID,
		"scope":     scope,
	}
	tokenJSON, _ := json.Marshal(tokenData)
	tokenKey := "oidc_token:" + accessToken
	if err := rdb.Set(context.Background(), tokenKey, tokenJSON, 1*time.Hour).Err(); err != nil {
		writeOIDCError(w, "server_error", "", http.StatusInternalServerError)
		return
	}

	// 签发 id_token (JWT)
	now := time.Now()
	claims := map[string]interface{}{
		"iss":   oidcIssuer(),
		"sub":   fmt.Sprintf("%d", userID),
		"aud":   clientID,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"email": user.Email,
		"name":  user.DisplayName,
	}
	if nonce != "" && nonce != "<nil>" {
		claims["nonce"] = nonce
	}

	idToken, err := signJWT(claims)
	if err != nil {
		log.Printf("oidc id_token sign failed: %v", err)
		writeOIDCError(w, "server_error", "", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
		"scope":        scope,
	})
}

// GET /oauth/userinfo
func OIDCUserInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 从 Authorization header 取 access_token
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
		return
	}
	accessToken := strings.TrimSpace(auth[len("Bearer "):])

	// 从 Redis 读 token 数据
	tokenKey := "oidc_token:" + accessToken
	tokenJSON, err := rdb.Get(context.Background(), tokenKey).Bytes()
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
		return
	}

	var tokenData map[string]interface{}
	if err := json.Unmarshal(tokenJSON, &tokenData); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
		return
	}

	userID := int64(tokenData["user_id"].(float64))
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"sub":   fmt.Sprintf("%d", user.ID),
		"email": user.Email,
		"name":  user.DisplayName,
	})
}

// --- 工具函数 ---

func isValidRedirectURI(client *models.OAuthClient, uri string) bool {
	for _, allowed := range client.RedirectURIs {
		if allowed == uri {
			return true
		}
	}
	return false
}

// 支持 POST body 和 HTTP Basic Auth 两种方式传 client 凭证
func extractClientCredentials(r *http.Request) (string, string) {
	// 先查 POST body
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID != "" && clientSecret != "" {
		return clientID, clientSecret
	}

	// 查 Basic Auth
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(auth[6:])
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}

	return clientID, clientSecret
}

// RS256 JWT 签名
func signJWT(claims map[string]interface{}) (string, error) {
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": oidcKeyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, oidcPrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + sigB64, nil
}

func writeOIDCError(w http.ResponseWriter, errorCode, description string, status int) {
	w.WriteHeader(status)
	resp := map[string]string{"error": errorCode}
	if description != "" {
		resp["error_description"] = description
	}
	_ = json.NewEncoder(w).Encode(resp)
}
