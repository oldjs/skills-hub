package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"skills-hub/db"
	"skills-hub/models"

	"github.com/redis/go-redis/v9"
)

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
			log.Printf("读取会话失败: %v", err)
		}
		return nil
	}

	var sess sessionData
	if err := json.Unmarshal(data, &sess); err != nil {
		log.Printf("解析会话失败: %v", err)
		return nil
	}

	return &sess
}

func deleteSession(token string) {
	if err := rdb.Del(context.Background(), sessionKey(token)).Err(); err != nil && err != redis.Nil {
		log.Printf("删除会话失败: %v", err)
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
	if err != nil && err != redis.Nil {
		log.Printf("校验 CSRF 失败: %v", err)
	}
	return err == nil && owner == csrfOwner(r)
}

func csrfOwner(r *http.Request) string {
	if token := sessionTokenFromRequest(r); token != "" {
		return "session:" + token
	}
	return "guest"
}

func IsLoggedIn(r *http.Request) bool {
	return getSession(r) != nil
}

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

func GetCurrentSession(r *http.Request) *sessionData {
	return getSession(r)
}

func GetClientIP(r *http.Request) string {
	if os.Getenv("TRUST_PROXY_HEADERS") == "true" {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			return strings.TrimSpace(parts[0])
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func secondsRemaining(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int((d + time.Second - 1) / time.Second)
}

func rateLimitKey(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return prefix + hex.EncodeToString(sum[:])
}

func failureLockState(key string, maxFailures int) (bool, time.Duration, error) {
	cnt, err := rdb.Get(context.Background(), key).Int64()
	if err == redis.Nil {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if cnt < int64(maxFailures) {
		return false, 0, nil
	}
	remaining, err := rdb.TTL(context.Background(), key).Result()
	if err != nil {
		return true, 0, err
	}
	return true, remaining, nil
}

func recordFailure(key string, ttl time.Duration) error {
	ctx := context.Background()
	cnt, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if cnt == 1 {
		return rdb.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func clearFailures(key string) error {
	err := rdb.Del(context.Background(), key).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

func shouldBePlatformAdmin(email string) bool {
	adminEmails := strings.Split(os.Getenv("PLATFORM_ADMIN_EMAILS"), ",")
	email = strings.ToLower(strings.TrimSpace(email))
	for _, item := range adminEmails {
		if strings.ToLower(strings.TrimSpace(item)) == email && email != "" {
			return true
		}
	}
	return false
}

func ensureUserTenant(user *models.User) (*models.UserTenant, error) {
	if err := db.AcceptPendingInvites(user.ID, user.Email); err != nil {
		return nil, err
	}

	tenant, err := db.PickActiveTenant(user.ID, user.LastTenantID)
	if err != nil {
		return nil, err
	}
	if tenant != nil {
		return tenant, nil
	}

	personalTenant, err := db.CreatePersonalTenantForUser(user.ID, user.DisplayName, user.Email)
	if err != nil {
		return nil, err
	}
	return db.GetUserTenant(user.ID, personalTenant.ID)
}

func buildSession(user *models.User, tenant *models.UserTenant) sessionData {
	return sessionData{
		UserID:          user.ID,
		Email:           user.Email,
		DisplayName:     user.DisplayName,
		IsPlatformAdmin: user.IsPlatformAdmin,
		IsSubAdmin:      user.IsSubAdmin,
		CurrentTenantID: tenant.TenantID,
		TenantName:      tenant.TenantName,
		TenantSlug:      tenant.TenantSlug,
		TenantRole:      tenant.TenantRole,
	}
}

func createUserSession(w http.ResponseWriter, user *models.User, tenant *models.UserTenant) error {
	token, err := generateToken()
	if err != nil {
		return err
	}
	if err := setSession(token, buildSession(user, tenant)); err != nil {
		return err
	}
	setSessionCookie(w, token)
	return nil
}

func loginPageData(title string) map[string]interface{} {
	return map[string]interface{}{
		"Title": title,
	}
}

func UserLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := loadActiveSession(w, r)
	if err != nil {
		logSessionRefreshError(err)
		http.Error(w, "系统繁忙，请稍后重试", http.StatusInternalServerError)
		return
	}
	if sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		data := loginPageData("登录 - Skills Hub")
		// OAuth 回调可能带 error 参数
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			data["Error"] = errMsg
		}
		RenderTemplate(w, r, "login.html", data)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	email := normalizeEmail(r.FormValue("email"))
	code := strings.TrimSpace(r.FormValue("code"))
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	data := loginPageData("登录 - Skills Hub")
	data["Email"] = email

	if email == "" || code == "" {
		data["Error"] = "请输入邮箱和验证码"
		RenderTemplate(w, r, "login.html", data)
		return
	}

	// 校验图形验证码
	if !validateCaptcha(r, captchaInput) {
		data["Error"] = "图形验证码错误，请重新输入"
		RenderTemplate(w, r, "login.html", data)
		return
	}

	user, err := db.GetUserByEmail(email)
	if err != nil {
		data["Error"] = "系统繁忙，请稍后重试"
		RenderTemplate(w, r, "login.html", data)
		return
	}
	if user == nil {
		data["Error"] = "该邮箱未注册，请先创建账号"
		RenderTemplate(w, r, "login.html", data)
		return
	}
	if user.Status != "active" {
		data["Error"] = "账户已被禁用，请联系管理员"
		RenderTemplate(w, r, "login.html", data)
		return
	}

	if shouldBePlatformAdmin(user.Email) && !user.IsPlatformAdmin {
		if err := db.SetUserPlatformAdmin(user.ID, true); err == nil {
			user.IsPlatformAdmin = true
			user.IsSubAdmin = false
		}
	}

	if ok, msg := verifyCode(r, user.Email, code, "login"); !ok {
		data["Error"] = msg
		RenderTemplate(w, r, "login.html", data)
		return
	}

	tenant, err := ensureUserTenant(user)
	if err != nil {
		data["Error"] = "登录失败，请稍后重试"
		RenderTemplate(w, r, "login.html", data)
		return
	}

	if err := db.UpdateUserLogin(user.ID, tenant.TenantID); err != nil {
		data["Error"] = "登录失败，请稍后重试"
		RenderTemplate(w, r, "login.html", data)
		return
	}
	if err := createUserSession(w, user, tenant); err != nil {
		data["Error"] = "登录失败，请稍后重试"
		RenderTemplate(w, r, "login.html", data)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func UserRegister(w http.ResponseWriter, r *http.Request) {
	sess, err := loadActiveSession(w, r)
	if err != nil {
		logSessionRefreshError(err)
		http.Error(w, "系统繁忙，请稍后重试", http.StatusInternalServerError)
		return
	}
	if sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		RenderTemplate(w, r, "register.html", map[string]interface{}{
			"Title": "注册 - Skills Hub",
		})
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	email := normalizeEmail(r.FormValue("email"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	code := strings.TrimSpace(r.FormValue("code"))
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	data := map[string]interface{}{
		"Title":       "注册 - Skills Hub",
		"Email":       email,
		"DisplayName": displayName,
	}

	if displayName == "" || email == "" || code == "" {
		data["Error"] = "请填写完整信息"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	// 校验图形验证码
	if !validateCaptcha(r, captchaInput) {
		data["Error"] = "图形验证码错误，请重新输入"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	if len([]rune(displayName)) < 2 || len([]rune(displayName)) > 32 {
		data["Error"] = "显示名称长度需要 2-32 个字符"
		RenderTemplate(w, r, "register.html", data)
		return
	}
	if !validateEmail(email) {
		data["Error"] = "邮箱格式不正确"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	existingUser, err := db.GetUserByEmail(email)
	if err != nil {
		data["Error"] = "系统繁忙，请稍后重试"
		RenderTemplate(w, r, "register.html", data)
		return
	}
	if existingUser != nil {
		data["Error"] = "该邮箱已注册，请直接登录"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	if ok, msg := verifyCode(r, email, code, "register"); !ok {
		data["Error"] = msg
		RenderTemplate(w, r, "register.html", data)
		return
	}

	user, err := db.CreateUser(email, displayName, shouldBePlatformAdmin(email))
	if err != nil {
		data["Error"] = "注册失败，请稍后重试"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	tenant, err := ensureUserTenant(user)
	if err != nil {
		data["Error"] = "注册失败，请稍后重试"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	if err := db.UpdateUserLogin(user.ID, tenant.TenantID); err != nil {
		data["Error"] = "注册成功，但登录失败，请稍后重试"
		RenderTemplate(w, r, "register.html", data)
		return
	}
	if err := createUserSession(w, user, tenant); err != nil {
		data["Error"] = "注册成功，但登录失败，请稍后重试"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func UserLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
		deleteSession(cookie.Value)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func SwitchTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	sess := getSession(r)
	if sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tenantID, err := parseInt64(r.FormValue("tenant_id"))
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tenant, err := db.GetUserTenant(sess.UserID, tenantID)
	if err != nil || tenant == nil || tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, err := db.GetUserByID(sess.UserID)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := db.UpdateUserLastTenant(user.ID, tenant.TenantID); err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := setSession(cookie.Value, buildSession(user, tenant)); err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, safeLocalRedirect(r.Referer(), r.Host), http.StatusSeeOther)
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := loadActiveSession(w, r)
		if err != nil {
			logSessionRefreshError(err)
			http.Error(w, "系统繁忙，请稍后重试", http.StatusInternalServerError)
			return
		}
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func RequirePlatformAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := loadActiveSession(w, r)
		if err != nil {
			logSessionRefreshError(err)
			http.Error(w, "系统繁忙，请稍后重试", http.StatusInternalServerError)
			return
		}
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !sess.IsPlatformAdmin {
			http.Error(w, "权限不足", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := loadActiveSession(w, r)
		if err != nil {
			logSessionRefreshError(err)
			http.Error(w, "系统繁忙，请稍后重试", http.StatusInternalServerError)
			return
		}
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !sess.IsPlatformAdmin && !sess.IsSubAdmin {
			http.Error(w, "需要管理员权限", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func safeLocalRedirect(referrer, host string) string {
	if strings.TrimSpace(referrer) == "" {
		return "/"
	}

	parsed, err := url.Parse(referrer)
	if err != nil {
		return "/"
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, host) {
		return "/"
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return "/"
	}
	if parsed.RawQuery == "" {
		return parsed.Path
	}
	return parsed.Path + "?" + parsed.RawQuery
}
