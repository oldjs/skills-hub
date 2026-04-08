package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"skills-hub/db"
)

// OAuth 配置
type oauthProviderConfig struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	Scopes      string
	ClientID    string
	ClientSecret string
}

func githubConfig() oauthProviderConfig {
	return oauthProviderConfig{
		AuthURL:      "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
		Scopes:       "user:email",
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}
}

func googleConfig() oauthProviderConfig {
	return oauthProviderConfig{
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:       "openid email profile",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
}

func oauthRedirectBase() string {
	base := os.Getenv("OAUTH_REDIRECT_BASE")
	if base == "" {
		base = "http://localhost:8080"
	}
	return strings.TrimRight(base, "/")
}

// --- 跳转到第三方授权页 ---

// GET /auth/github
func OAuthGitHubHandler(w http.ResponseWriter, r *http.Request) {
	cfg := githubConfig()
	if cfg.ClientID == "" {
		http.Error(w, "GitHub OAuth 未配置", http.StatusServiceUnavailable)
		return
	}
	startOAuthFlow(w, r, "github", cfg)
}

// GET /auth/google
func OAuthGoogleHandler(w http.ResponseWriter, r *http.Request) {
	cfg := googleConfig()
	if cfg.ClientID == "" {
		http.Error(w, "Google OAuth 未配置", http.StatusServiceUnavailable)
		return
	}
	startOAuthFlow(w, r, "google", cfg)
}

func startOAuthFlow(w http.ResponseWriter, r *http.Request, provider string, cfg oauthProviderConfig) {
	// 生成 state 防 CSRF
	state, err := generateToken()
	if err != nil {
		http.Error(w, "系统错误", http.StatusInternalServerError)
		return
	}

	// state 存 Redis，5 分钟有效
	stateKey := "oauth_state:" + state
	if err := rdb.Set(context.Background(), stateKey, provider, 5*time.Minute).Err(); err != nil {
		http.Error(w, "系统错误", http.StatusInternalServerError)
		return
	}

	redirectURI := oauthRedirectBase() + "/auth/callback/" + provider
	authURL := cfg.AuthURL + "?" + url.Values{
		"client_id":     {cfg.ClientID},
		"redirect_uri":  {redirectURI},
		"scope":         {cfg.Scopes},
		"state":         {state},
		"response_type": {"code"},
	}.Encode()

	http.Redirect(w, r, authURL, http.StatusFound)
}

// --- 回调处理 ---

// GET /auth/callback/github
func OAuthCallbackGitHubHandler(w http.ResponseWriter, r *http.Request) {
	cfg := githubConfig()
	handleOAuthCallback(w, r, "github", cfg)
}

// GET /auth/callback/google
func OAuthCallbackGoogleHandler(w http.ResponseWriter, r *http.Request) {
	cfg := googleConfig()
	handleOAuthCallback(w, r, "google", cfg)
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, provider string, cfg oauthProviderConfig) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		redirectLoginError(w, r, "授权失败，请重试")
		return
	}

	// 校验 state
	stateKey := "oauth_state:" + state
	storedProvider, err := rdb.GetDel(context.Background(), stateKey).Result()
	if err != nil || storedProvider != provider {
		redirectLoginError(w, r, "授权请求已过期，请重试")
		return
	}

	// 用 code 换 access_token
	accessToken, err := exchangeCodeForToken(provider, cfg, code)
	if err != nil {
		log.Printf("oauth token exchange failed [%s]: %v", provider, err)
		redirectLoginError(w, r, "获取授权令牌失败，请重试")
		return
	}

	// 拿用户信息
	providerUserID, email, displayName, err := fetchOAuthUserInfo(provider, cfg, accessToken)
	if err != nil {
		log.Printf("oauth userinfo fetch failed [%s]: %v", provider, err)
		redirectLoginError(w, r, "获取用户信息失败，请重试")
		return
	}

	if email == "" {
		redirectLoginError(w, r, "未获取到邮箱信息，请确保授权了邮箱权限")
		return
	}
	email = normalizeEmail(email)

	// 查 oauth_connections 看这个第三方账号是否已经绑定过
	conn, err := db.GetOAuthConnection(provider, providerUserID)
	if err != nil {
		log.Printf("oauth connection lookup failed: %v", err)
		redirectLoginError(w, r, "系统错误，请稍后重试")
		return
	}

	var userID int64

	if conn != nil {
		// 已有关联，直接用绑定的 user_id
		userID = conn.UserID
	} else {
		// 没有关联，按邮箱查用户（同一邮箱自动合并）
		user, err := db.GetUserByEmail(email)
		if err != nil {
			log.Printf("oauth user lookup failed: %v", err)
			redirectLoginError(w, r, "系统错误，请稍后重试")
			return
		}
		if user != nil {
			userID = user.ID
		} else {
			// 新用户：自动注册
			if displayName == "" {
				displayName = strings.Split(email, "@")[0]
			}
			newUser, err := db.CreateUser(email, displayName, shouldBePlatformAdmin(email))
			if err != nil {
				log.Printf("oauth user create failed: %v", err)
				redirectLoginError(w, r, "注册失败，请稍后重试")
				return
			}
			userID = newUser.ID
		}
	}

	// 保存/更新 OAuth 关联
	if err := db.CreateOAuthConnection(userID, provider, providerUserID, email, displayName, accessToken, ""); err != nil {
		log.Printf("oauth connection save failed: %v", err)
		// 不阻断登录流程，只记日志
	}

	// 拿用户对象，走正常的 session 创建流程
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		redirectLoginError(w, r, "登录失败，请稍后重试")
		return
	}
	if user.Status != "active" {
		redirectLoginError(w, r, "账户已被禁用，请联系管理员")
		return
	}

	// 环境变量配置的 admin 提权
	if shouldBePlatformAdmin(user.Email) && !user.IsPlatformAdmin {
		if err := db.SetUserPlatformAdmin(user.ID, true); err == nil {
			user.IsPlatformAdmin = true
		}
	}

	tenant, err := ensureUserTenant(user)
	if err != nil {
		redirectLoginError(w, r, "登录失败，请稍后重试")
		return
	}

	_ = db.UpdateUserLogin(user.ID, tenant.TenantID)
	if err := createUserSession(w, user, tenant); err != nil {
		redirectLoginError(w, r, "登录失败，请稍后重试")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func redirectLoginError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

// --- Token 交换 ---

func exchangeCodeForToken(provider string, cfg oauthProviderConfig, code string) (string, error) {
	redirectURI := oauthRedirectBase() + "/auth/callback/" + provider
	data := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	// Google 需要 grant_type
	if provider == "google" {
		data.Set("grant_type", "authorization_code")
	}

	req, err := http.NewRequest(http.MethodPost, cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("token response parse failed: %s", string(body))
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}

	return tokenResp.AccessToken, nil
}

// --- 用户信息获取 ---

func fetchOAuthUserInfo(provider string, cfg oauthProviderConfig, accessToken string) (providerUserID, email, displayName string, err error) {
	switch provider {
	case "github":
		return fetchGitHubUser(accessToken)
	case "google":
		return fetchGoogleUser(cfg.UserInfoURL, accessToken)
	default:
		return "", "", "", fmt.Errorf("unknown provider: %s", provider)
	}
}

func fetchGitHubUser(accessToken string) (string, string, string, error) {
	// 拿基本信息
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	var userInfo struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", "", "", err
	}

	displayName := userInfo.Name
	if displayName == "" {
		displayName = userInfo.Login
	}
	providerUserID := fmt.Sprintf("%d", userInfo.ID)

	// GitHub 邮箱可能为空（设置了隐私），需要额外请求
	email := userInfo.Email
	if email == "" {
		email, _ = fetchGitHubPrimaryEmail(accessToken)
	}

	return providerUserID, email, displayName, nil
}

// GitHub 用户可能隐藏了邮箱，需要用 /user/emails 接口拿
func fetchGitHubPrimaryEmail(accessToken string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// 优先取 primary + verified
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	// 退而求其次，取第一个 verified
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified email found")
}

func fetchGoogleUser(userInfoURL, accessToken string) (string, string, string, error) {
	req, _ := http.NewRequest(http.MethodGet, userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	var userInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", "", "", err
	}

	return userInfo.ID, userInfo.Email, userInfo.Name, nil
}
