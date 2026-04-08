package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"skills-hub/db"
	"skills-hub/models"

	"github.com/redis/go-redis/v9"
)

var (
	integrationServer   *httptest.Server
	testRedisCommand    *exec.Cmd
	testWorkspaceDir    string
	testRedisAddress    string
	testSQLitePath      string
	csrfInputPattern    = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	apiKeyValuePattern  = regexp.MustCompile(`shk_[a-f0-9]+`)
	commentReplyTimeout = 2 * time.Second
)

func TestMain(m *testing.M) {
	code := 1

	if err := bootIntegrationEnvironment(); err == nil {
		code = m.Run()
	} else {
		fmt.Fprintf(os.Stderr, "boot integration environment failed: %v\n", err)
	}

	shutdownIntegrationEnvironment()
	os.Exit(code)
}

func bootIntegrationEnvironment() error {
	// 每次测试都放到独立临时目录，脏数据和上传文件都不会串。
	tempDir, err := os.MkdirTemp("", "skills-hub-handlers-test-")
	if err != nil {
		return err
	}
	testWorkspaceDir = tempDir
	testSQLitePath = filepath.Join(tempDir, "skills-hub-test.sqlite")

	// 测试里直接拉起一个真 Redis，验证码、会话和限流都走真实链路。
	redisAddress, cmd, err := startRedisProcess(tempDir)
	if err != nil {
		return err
	}
	testRedisAddress = redisAddress
	testRedisCommand = cmd

	if err := os.Setenv("REDIS_URL", testRedisAddress); err != nil {
		return err
	}
	if err := os.Setenv("COOKIE_SECURE", "false"); err != nil {
		return err
	}
	if err := os.Setenv("SITE_URL", "http://skills-hub.test"); err != nil {
		return err
	}
	if err := db.Init(testSQLitePath); err != nil {
		return err
	}
	if err := InitAuth(); err != nil {
		return err
	}
	db.SetCacheClient(GetRedisClient())
	InitTemplates("../templates")

	// 测试服务器只挂业务测试要用到的路由，尽量贴近真实入口。
	integrationServer = httptest.NewServer(newIntegrationMux())
	return nil
}

func shutdownIntegrationEnvironment() {
	if integrationServer != nil {
		integrationServer.Close()
	}
	CloseAuth()
	db.Close()
	stopRedisProcess(testRedisCommand)
	if testWorkspaceDir != "" {
		_ = os.RemoveAll(testWorkspaceDir)
	}
	_ = os.RemoveAll("./uploads")
}

func newIntegrationMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", OptionalAuth(HomeHandler))
	mux.HandleFunc("/login", UserLogin)
	mux.HandleFunc("/register", UserRegister)
	mux.HandleFunc("/logout", UserLogout)
	mux.HandleFunc("/captcha", CaptchaHandler)
	mux.HandleFunc("/send-code", SendCodeHandler)
	mux.HandleFunc("/account", RequireAuth(AccountHandler))
	mux.HandleFunc("/account/api-keys/create", RequireAuth(AccountCreateAPIKeyHandler))
	mux.HandleFunc("/search", OptionalAuth(SearchHandler))
	mux.HandleFunc("/api/search", RequireAuth(SearchAPIHandler))
	mux.HandleFunc("/leaderboard", OptionalAuth(LeaderboardHandler))
	mux.HandleFunc("/skill", OptionalAuth(SkillHandler))
	mux.HandleFunc("/api/v1/upload", APIV1UploadHandler)
	mux.HandleFunc("/api/v1/skills/", APIV1SkillDetailHandler)
	mux.HandleFunc("/api/v1/download/", APIV1DownloadHandler)
	mux.HandleFunc("/api/rate", RequireAuth(RateSkillHandler))
	mux.HandleFunc("/api/comment", RequireAuth(CommentSkillHandler))
	mux.HandleFunc("/collections/create", RequireAuth(CollectionCreateHandler))
	mux.HandleFunc("/collections/add-skill", RequireAuth(CollectionAddSkillHandler))
	mux.HandleFunc("/collections/delete", RequireAuth(CollectionDeleteHandler))
	mux.HandleFunc("/admin/skill", RequireAdmin(AdminSkillDetailHandler))
	mux.HandleFunc("/admin/skill/review", RequireAdmin(AdminSkillReviewHandler))
	return mux
}

func startRedisProcess(tempDir string) (string, *exec.Cmd, error) {
	// 先抢一个空闲端口，后面 redis 直接绑过去。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	command := exec.Command(
		"redis-server",
		"--save", "",
		"--appendonly", "no",
		"--bind", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--dir", tempDir,
	)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return "", nil, err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return address, command, nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	stopRedisProcess(command)
	return "", nil, fmt.Errorf("redis did not become ready at %s", address)
}

func stopRedisProcess(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}

	_ = command.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = command.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		<-done
	}
}

func resetIntegrationState(t *testing.T) {
	t.Helper()

	// 先把 Redis 清空，验证码、限流、session 都从干净状态开始。
	if err := GetRedisClient().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}

	// SQLite 走整库清理，测试之间就不会互相污染。
	tables := []string{
		"collection_items",
		"skill_collections",
		"comment_votes",
		"skill_comments",
		"skill_ratings",
		"skill_bookmarks",
		"notifications",
		"skill_versions",
		"admin_action_logs",
		"api_keys",
		"sync_log",
		"skills_fts",
		"skills",
		"tenant_invites",
		"tenant_members",
		"users",
		"tenants",
	}
	for _, table := range tables {
		if _, err := db.GetDB().Exec(`DELETE FROM ` + table); err != nil {
			t.Fatalf("clear table %s: %v", table, err)
		}
	}
	if _, err := db.GetDB().Exec(`DELETE FROM sqlite_sequence`); err != nil {
		t.Fatalf("reset sqlite sequence: %v", err)
	}
	if err := os.RemoveAll("./uploads"); err != nil {
		t.Fatalf("clear uploads: %v", err)
	}
}

func newBrowserClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}

	// 禁掉自动跟随，业务链路里的 303/302 我们自己断言更稳。
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func createUserWithTenant(t *testing.T, email, displayName string, isPlatformAdmin bool) (*models.User, *models.UserTenant) {
	t.Helper()

	user, err := db.CreateUser(email, displayName, isPlatformAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	tenant, err := ensureUserTenant(user)
	if err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}
	if err := db.UpdateUserLogin(user.ID, tenant.TenantID); err != nil {
		t.Fatalf("update user login: %v", err)
	}
	user, err = db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user, tenant
}

func newAuthenticatedClient(t *testing.T, user *models.User, tenant *models.UserTenant) *http.Client {
	t.Helper()

	client := newBrowserClient(t)
	token, err := generateToken()
	if err != nil {
		t.Fatalf("generate session token: %v", err)
	}
	if err := setSession(token, buildSession(user, tenant)); err != nil {
		t.Fatalf("store session: %v", err)
	}

	serverURL, err := url.Parse(integrationServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	client.Jar.SetCookies(serverURL, []*http.Cookie{{
		Name:  "session",
		Value: token,
		Path:  "/",
	}})
	return client
}

func mustGetCSRFToken(t *testing.T, client *http.Client, path string) string {
	t.Helper()

	response := mustRequest(t, client, http.MethodGet, path, nil, nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get csrf page %s: status %d", path, response.StatusCode)
	}
	body := mustReadBody(t, response)
	match := csrfInputPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("csrf token not found in %s", path)
	}
	return match[1]
}

func mustFetchCaptchaAnswer(t *testing.T, client *http.Client) string {
	t.Helper()

	response := mustRequest(t, client, http.MethodGet, "/captcha", nil, nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("captcha request failed: status %d", response.StatusCode)
	}

	serverURL, err := url.Parse(integrationServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	for _, cookie := range client.Jar.Cookies(serverURL) {
		if cookie.Name != "captcha_id" || cookie.Value == "" {
			continue
		}
		answer, err := GetRedisClient().Get(context.Background(), captchaRedisKey(cookie.Value)).Result()
		if err != nil {
			t.Fatalf("read captcha answer: %v", err)
		}
		return answer
	}
	t.Fatal("captcha cookie not found")
	return ""
}

func mustReadEmailCode(t *testing.T, email, purpose string) string {
	t.Helper()

	code, err := GetRedisClient().Get(context.Background(), emailCodeKey(email, purpose)).Result()
	if err != nil {
		t.Fatalf("read email code: %v", err)
	}
	return code
}

func mustCreateAPIKey(t *testing.T, client *http.Client) string {
	t.Helper()

	csrfToken := mustGetCSRFToken(t, client, "/account")
	response := mustFormPost(t, client, "/account/api-keys/create", url.Values{
		"csrf_token": {csrfToken},
		"name":       {"integration-bot"},
	}, nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("create api key: status %d", response.StatusCode)
	}
	body := mustReadBody(t, response)
	match := apiKeyValuePattern.FindString(body)
	if match == "" {
		t.Fatal("generated api key not found in account page")
	}
	return match
}

func mustRequest(t *testing.T, client *http.Client, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()

	request, err := http.NewRequest(method, integrationServer.URL+path, body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	return response
}

func mustFormPost(t *testing.T, client *http.Client, path string, values url.Values, headers map[string]string) *http.Response {
	t.Helper()

	if headers == nil {
		headers = map[string]string{}
	}
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	return mustRequest(t, client, http.MethodPost, path, strings.NewReader(values.Encode()), headers)
}

func mustReadBody(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}

func mustExec(t *testing.T, query string, args ...interface{}) sql.Result {
	t.Helper()

	result, err := db.GetDB().Exec(query, args...)
	if err != nil {
		t.Fatalf("exec query failed: %v", err)
	}
	return result
}

func mustInsertApprovedSkill(t *testing.T, tenantID int64, slug, displayName, summary, content, version, categories, author, source string, score float64, downloadCount int) int64 {
	t.Helper()

	now := time.Now()
	result := mustExec(t, `
		INSERT INTO skills (
			tenant_id, slug, display_name, summary, content, score, source_updated_at,
			version, categories, author, download_count, source, review_status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'approved', ?, ?)
	`, tenantID, slug, displayName, summary, content, score, now.Unix(), version, categories, author, downloadCount, source, now, now)
	skillID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted skill id: %v", err)
	}
	db.SyncSkillToFTS(skillID)
	return skillID
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func mustJSONDecode[T any](t *testing.T, response *http.Response, target *T) {
	t.Helper()

	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode json response: %v", err)
	}
}

func requireCookieValue(t *testing.T, client *http.Client, cookieName string) string {
	t.Helper()

	serverURL, err := url.Parse(integrationServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	for _, cookie := range client.Jar.Cookies(serverURL) {
		if cookie.Name == cookieName {
			return cookie.Value
		}
	}
	t.Fatalf("cookie %s not found", cookieName)
	return ""
}

func waitForRedisKeyDeletion(t *testing.T, key string) {
	t.Helper()

	waitForCondition(t, commentReplyTimeout, func() bool {
		err := GetRedisClient().Get(context.Background(), key).Err()
		return errors.Is(err, redis.Nil)
	})
}
