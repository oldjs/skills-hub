package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"skills-hub/db"
	"skills-hub/models"
)

// 登录/注册防暴力破解：5 次失败锁定 15 分钟
const (
	maxLoginFailures = 5
	loginLockTTL     = 15 * time.Minute
)

// --- IP / 工具函数 ---

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

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func secondsRemaining(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int((d + time.Second - 1) / time.Second)
}

// --- 限流工具 ---

func rateLimitKey(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return prefix + hex.EncodeToString(sum[:])
}

func failureLockState(key string, maxFailures int) (bool, time.Duration, error) {
	cnt, err := rdb.Get(context.Background(), key).Int64()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, 0, nil
		}
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
	if err != nil && err.Error() == "redis: nil" {
		return nil
	}
	return err
}

// --- Auth 辅助 ---

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
	return map[string]interface{}{"Title": title}
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

// --- Handlers ---

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

	ip := GetClientIP(r)
	loginIPKey := rateLimitKey("ratelimit:login-ip:", ip)
	loginEmailKey := rateLimitKey("ratelimit:login-email:", normalizeEmail(email))
	if locked, remaining, _ := failureLockState(loginIPKey, maxLoginFailures); locked {
		data["Error"] = fmt.Sprintf("登录尝试次数过多，请 %d 秒后重试", secondsRemaining(remaining))
		RenderTemplate(w, r, "login.html", data)
		return
	}
	if locked, remaining, _ := failureLockState(loginEmailKey, maxLoginFailures); locked {
		data["Error"] = fmt.Sprintf("该邮箱登录尝试次数过多，请 %d 秒后重试", secondsRemaining(remaining))
		RenderTemplate(w, r, "login.html", data)
		return
	}

	if !validateCaptcha(r, captchaInput) {
		_ = recordFailure(loginIPKey, loginLockTTL)
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
		_ = recordFailure(loginIPKey, loginLockTTL)
		_ = recordFailure(loginEmailKey, loginLockTTL)
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
		_ = recordFailure(loginIPKey, loginLockTTL)
		_ = recordFailure(loginEmailKey, loginLockTTL)
		data["Error"] = msg
		RenderTemplate(w, r, "login.html", data)
		return
	}
	_ = clearFailures(loginIPKey)
	_ = clearFailures(loginEmailKey)

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
		RenderTemplate(w, r, "register.html", map[string]interface{}{"Title": "注册 - Skills Hub"})
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
		"Title": "注册 - Skills Hub", "Email": email, "DisplayName": displayName,
	}

	if displayName == "" || email == "" || code == "" {
		data["Error"] = "请填写完整信息"
		RenderTemplate(w, r, "register.html", data)
		return
	}

	regIPKey := rateLimitKey("ratelimit:register-ip:", GetClientIP(r))
	if locked, remaining, _ := failureLockState(regIPKey, maxLoginFailures); locked {
		data["Error"] = fmt.Sprintf("注册尝试次数过多，请 %d 秒后重试", secondsRemaining(remaining))
		RenderTemplate(w, r, "register.html", data)
		return
	}

	if !validateCaptcha(r, captchaInput) {
		_ = recordFailure(regIPKey, loginLockTTL)
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
	if ok, msg := validateEmailForRegistration(email); !ok {
		data["Error"] = msg
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
		_ = recordFailure(regIPKey, loginLockTTL)
		data["Error"] = msg
		RenderTemplate(w, r, "register.html", data)
		return
	}
	_ = clearFailures(regIPKey)

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

// --- Middleware ---

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

func OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := loadActiveSession(w, r); err != nil {
			logSessionRefreshError(err)
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
