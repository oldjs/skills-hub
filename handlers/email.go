package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"skills-hub/db"

	"github.com/redis/go-redis/v9"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const (
	maxVerificationFailures = 5
	verificationLockTTL     = 10 * time.Minute
	emailCodeTTL            = 5 * time.Minute
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func emailCodeKey(email, purpose string) string {
	return "email_code:" + purpose + ":" + normalizeEmail(email)
}

func rateLimitEmailKey(email string) string {
	return "ratelimit:email:" + normalizeEmail(email)
}

func rateLimitIPKey(ip string) string {
	return "ratelimit:ip:" + ip
}

func checkRateLimit(email, ip string) (bool, string, error) {
	ctx := context.Background()
	ttl, err := rdb.TTL(ctx, rateLimitEmailKey(email)).Result()
	if err != nil && err != redis.Nil {
		return false, "", err
	}
	if ttl > 0 {
		return false, fmt.Sprintf("发送太频繁，请 %d 秒后重试", int(ttl.Seconds())), nil
	}

	cnt, err := rdb.Get(ctx, rateLimitIPKey(ip)).Int64()
	if err != nil && err != redis.Nil {
		return false, "", err
	}
	if cnt >= 5 {
		return false, "请求过于频繁，请稍后再试", nil
	}
	return true, "", nil
}

func recordRateLimit(email, ip string) error {
	ctx := context.Background()
	if err := rdb.Set(ctx, rateLimitEmailKey(email), "1", 60*time.Second).Err(); err != nil {
		return err
	}
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, rateLimitIPKey(ip))
	pipe.Expire(ctx, rateLimitIPKey(ip), 60*time.Second)
	_, err := pipe.Exec(ctx)
	return err
}

func sendVerificationEmail(toEmail, code string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("RESEND_API_KEY 未配置，验证码 %s -> %s", code, toEmail)
		return nil
	}

	mailFrom := os.Getenv("MAIL_FROM")
	if mailFrom == "" {
		mailFrom = "noreply@example.com"
	}

	payload := map[string]interface{}{
		"from":    mailFrom,
		"to":      []string{toEmail},
		"subject": "Skills Hub 登录验证码",
		"html":    fmt.Sprintf("<div style=\"font-family:sans-serif;max-width:420px;margin:0 auto;padding:24px\"><h2 style=\"color:#ea580c\">Skills Hub 验证码</h2><p>你的验证码是：</p><div style=\"font-size:32px;font-weight:700;letter-spacing:8px;color:#111827;padding:16px 0\">%s</div><p style=\"color:#64748b;font-size:14px\">验证码 5 分钟内有效，请勿泄露给他人。</p></div>", code),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}
	return nil
}

func SendCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if !ValidateCSRFToken(r) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求"})
		return
	}

	email := normalizeEmail(r.FormValue("email"))
	purpose := strings.TrimSpace(r.FormValue("purpose"))
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	if !validateCaptcha(r, captchaInput) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "图形验证码错误"})
		return
	}
	if purpose != "login" && purpose != "register" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求"})
		return
	}
	if !validateEmail(email) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "邮箱格式不正确"})
		return
	}

	user, err := db.GetUserByEmail(email)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	if purpose == "login" && user == nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "该邮箱未注册"})
		return
	}
	if purpose == "register" && user != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "该邮箱已注册，请直接登录"})
		return
	}

	ip := GetClientIP(r)
	ok, msg, err := checkRateLimit(email, ip)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
		return
	}

	code, err := generateVerificationCode()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	if err := rdb.Set(context.Background(), emailCodeKey(email, purpose), code, emailCodeTTL).Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	if err := recordRateLimit(email, ip); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	if err := sendVerificationEmail(email, code); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "邮件发送失败，请重试"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"ok": "验证码已发送"})
}

var verifyCodeScript = redis.NewScript(`
local key = KEYS[1]
local input = ARGV[1]
local answer = redis.call('GET', key)
if not answer then
    return 0
end
if input == answer then
    redis.call('DEL', key)
    return 1
end
return 0
`)

func verifyCode(r *http.Request, email, code, purpose string) (bool, string) {
	attemptKey := rateLimitKey("ratelimit:verify-code:", normalizeEmail(email), purpose, GetClientIP(r))
	locked, remaining, err := failureLockState(attemptKey, maxVerificationFailures)
	if err != nil {
		log.Printf("检查验证码失败限制失败: %v", err)
		return false, "系统繁忙，请稍后重试"
	}
	if locked {
		return false, fmt.Sprintf("验证码错误次数过多，请 %d 秒后重试", secondsRemaining(remaining))
	}

	result, err := verifyCodeScript.Run(context.Background(), rdb, []string{emailCodeKey(email, purpose)}, strings.TrimSpace(code)).Int()
	if err != nil {
		log.Printf("校验验证码失败: %v", err)
		return false, "系统繁忙，请稍后重试"
	}
	if result != 1 {
		if rateErr := recordFailure(attemptKey, verificationLockTTL); rateErr != nil {
			log.Printf("记录验证码失败次数失败: %v", rateErr)
		}
		return false, "验证码错误或已过期"
	}
	if err := clearFailures(attemptKey); err != nil {
		log.Printf("清理验证码失败次数失败: %v", err)
	}
	return true, ""
}
