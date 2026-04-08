package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"skills-hub/db"

	"github.com/redis/go-redis/v9"
)

func TestIntegrationAuthFlow(t *testing.T) {
	resetIntegrationState(t)

	client := newBrowserClient(t)
	registerEmail := normalizeEmail("agent.user@gmail.com")

	// 先拿游客态的 CSRF，再走验证码发送。
	registerCSRF := mustGetCSRFToken(t, client, "/register")
	registerCaptcha := mustFetchCaptchaAnswer(t, client)
	registerSendCode := mustFormPost(t, client, "/send-code", url.Values{
		"csrf_token": {registerCSRF},
		"email":      {registerEmail},
		"purpose":    {"register"},
		"captcha":    {registerCaptcha},
	}, nil)
	if registerSendCode.StatusCode != http.StatusOK {
		t.Fatalf("send register code: status %d", registerSendCode.StatusCode)
	}
	var registerSendCodeBody map[string]string
	mustJSONDecode(t, registerSendCode, &registerSendCodeBody)
	if registerSendCodeBody["ok"] == "" {
		t.Fatalf("unexpected register send-code response: %#v", registerSendCodeBody)
	}

	registerCode := mustReadEmailCode(t, registerEmail, "register")
	registerCaptcha = mustFetchCaptchaAnswer(t, client)
	registerResponse := mustFormPost(t, client, "/register", url.Values{
		"csrf_token":   {registerCSRF},
		"email":        {registerEmail},
		"display_name": {"Agent User"},
		"code":         {registerCode},
		"captcha":      {registerCaptcha},
	}, nil)
	defer registerResponse.Body.Close()
	if registerResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("register: status %d", registerResponse.StatusCode)
	}
	if location := registerResponse.Header.Get("Location"); location != "/" {
		t.Fatalf("register redirect = %q, want /", location)
	}
	if sessionCookie := requireCookieValue(t, client, "session"); sessionCookie == "" {
		t.Fatal("session cookie missing after register")
	}
	if err := GetRedisClient().Get(context.Background(), emailCodeKey(registerEmail, "register")).Err(); err != redis.Nil {
		t.Fatalf("register code should be consumed, got err = %v", err)
	}

	user, err := db.GetUserByEmail(registerEmail)
	if err != nil {
		t.Fatalf("load registered user: %v", err)
	}
	if user == nil {
		t.Fatal("registered user not found in database")
	}
	tenant, err := db.PickActiveTenant(user.ID, user.LastTenantID)
	if err != nil {
		t.Fatalf("pick user tenant: %v", err)
	}
	if tenant == nil {
		t.Fatal("personal tenant not created during register flow")
	}

	// 登录态下重新拿一次 CSRF，后面的退出登录才会认这个 owner。
	logoutCSRF := mustGetCSRFToken(t, client, "/account")
	logoutResponse := mustFormPost(t, client, "/logout", url.Values{
		"csrf_token": {logoutCSRF},
	}, nil)
	defer logoutResponse.Body.Close()
	if logoutResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout: status %d", logoutResponse.StatusCode)
	}
	if location := logoutResponse.Header.Get("Location"); location != "/login" {
		t.Fatalf("logout redirect = %q, want /login", location)
	}

	accountAfterLogout := mustRequest(t, client, http.MethodGet, "/account", nil, nil)
	defer accountAfterLogout.Body.Close()
	if accountAfterLogout.StatusCode != http.StatusSeeOther || accountAfterLogout.Header.Get("Location") != "/login" {
		t.Fatalf("account after logout should redirect to login, got status=%d location=%q", accountAfterLogout.StatusCode, accountAfterLogout.Header.Get("Location"))
	}

	// 这条用例要把注册和登录串在一起测，不然这里会卡在真实的 60 秒发码频控上。
	if err := GetRedisClient().Del(context.Background(), rateLimitEmailKey(registerEmail), rateLimitIPKey("127.0.0.1")).Err(); err != nil {
		t.Fatalf("clear send-code rate limit keys: %v", err)
	}

	// 重新登录要重新拿游客态 CSRF 和验证码。
	loginCSRF := mustGetCSRFToken(t, client, "/login")
	loginCaptcha := mustFetchCaptchaAnswer(t, client)
	loginSendCode := mustFormPost(t, client, "/send-code", url.Values{
		"csrf_token": {loginCSRF},
		"email":      {registerEmail},
		"purpose":    {"login"},
		"captcha":    {loginCaptcha},
	}, nil)
	if loginSendCode.StatusCode != http.StatusOK {
		t.Fatalf("send login code: status %d", loginSendCode.StatusCode)
	}
	var loginSendCodeBody map[string]string
	mustJSONDecode(t, loginSendCode, &loginSendCodeBody)
	if loginSendCodeBody["ok"] == "" {
		t.Fatalf("unexpected login send-code response: %#v", loginSendCodeBody)
	}

	loginCode := mustReadEmailCode(t, registerEmail, "login")
	loginCaptcha = mustFetchCaptchaAnswer(t, client)
	loginResponse := mustFormPost(t, client, "/login", url.Values{
		"csrf_token": {loginCSRF},
		"email":      {registerEmail},
		"code":       {loginCode},
		"captcha":    {loginCaptcha},
	}, nil)
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: status %d", loginResponse.StatusCode)
	}
	if location := loginResponse.Header.Get("Location"); location != "/" {
		t.Fatalf("login redirect = %q, want /", location)
	}

	accountAfterLogin := mustRequest(t, client, http.MethodGet, "/account", nil, nil)
	defer accountAfterLogin.Body.Close()
	if accountAfterLogin.StatusCode != http.StatusOK {
		t.Fatalf("account after login: status %d", accountAfterLogin.StatusCode)
	}
	accountBody := mustReadBody(t, accountAfterLogin)
	if !strings.Contains(accountBody, "API Key 管理") {
		t.Fatalf("account page body missing expected content: %s", accountBody)
	}
	if err := GetRedisClient().Get(context.Background(), emailCodeKey(registerEmail, "login")).Err(); err != redis.Nil {
		t.Fatalf("login code should be consumed, got err = %v", err)
	}
}

func TestIntegrationSearchAndLeaderboard(t *testing.T) {
	resetIntegrationState(t)

	user, tenant := createUserWithTenant(t, "searcher@gmail.com", "Searcher", false)
	authClient := newAuthenticatedClient(t, user, tenant)
	anonClient := newBrowserClient(t)

	// 这几条技能把搜索、排序和排行榜都喂满。
	radarID := mustInsertApprovedSkill(t, tenant.TenantID, "radar-agent", "Radar Agent", "Find browser workflows fast", "# Radar Agent", "1.0.0", "搜索发现,自动化", "Radar Team", "clawhub", 0.8, 12)
	atlasID := mustInsertApprovedSkill(t, tenant.TenantID, "atlas-agent", "Atlas Agent", "Browser assistant for shared playbooks", "# Atlas Agent", "2.1.0", "搜索发现,协作", "Atlas Team", "clawhub", 0.7, 500)
	mustInsertApprovedSkill(t, tenant.TenantID, "quiet-tool", "Quiet Tool", "Low signal utility", "# Quiet Tool", "0.9.0", "实用工具", "Quiet Team", "clawhub", 0.2, 0)

	secondUser, _ := createUserWithTenant(t, "rater@qq.com", "Second Rater", false)
	if err := db.AddRating(tenant.TenantID, radarID, user.ID, 5); err != nil {
		t.Fatalf("add radar rating 1: %v", err)
	}
	if err := db.AddRating(tenant.TenantID, radarID, secondUser.ID, 5); err != nil {
		t.Fatalf("add radar rating 2: %v", err)
	}
	if err := db.AddRating(tenant.TenantID, atlasID, user.ID, 4); err != nil {
		t.Fatalf("add atlas rating 1: %v", err)
	}
	if err := db.AddRating(tenant.TenantID, atlasID, secondUser.ID, 4); err != nil {
		t.Fatalf("add atlas rating 2: %v", err)
	}
	if err := db.AddComment(tenant.TenantID, radarID, user.ID, "Radar comment one", nil); err != nil {
		t.Fatalf("add radar comment 1: %v", err)
	}
	if err := db.AddComment(tenant.TenantID, radarID, secondUser.ID, "Radar comment two", nil); err != nil {
		t.Fatalf("add radar comment 2: %v", err)
	}
	if err := db.AddComment(tenant.TenantID, atlasID, user.ID, "Atlas comment one", nil); err != nil {
		t.Fatalf("add atlas comment: %v", err)
	}

	searchResponse := mustRequest(t, anonClient, http.MethodGet, "/search?format=json&q=Agent&category=%E6%90%9C%E7%B4%A2%E5%8F%91%E7%8E%B0&sort=rating&min_rating=4", nil, nil)
	if searchResponse.StatusCode != http.StatusOK {
		t.Fatalf("search json: status %d", searchResponse.StatusCode)
	}
	var searchBody struct {
		Skills []struct {
			DisplayName string `json:"displayName"`
		} `json:"skills"`
		Total int `json:"total"`
	}
	mustJSONDecode(t, searchResponse, &searchBody)
	if searchBody.Total != 2 || len(searchBody.Skills) != 2 {
		t.Fatalf("search result size = total:%d len:%d, want 2", searchBody.Total, len(searchBody.Skills))
	}
	if searchBody.Skills[0].DisplayName != "Radar Agent" || searchBody.Skills[1].DisplayName != "Atlas Agent" {
		t.Fatalf("unexpected search order: %#v", searchBody.Skills)
	}

	quickSearchResponse := mustRequest(t, authClient, http.MethodGet, "/api/search?q=Agent", nil, nil)
	if quickSearchResponse.StatusCode != http.StatusOK {
		t.Fatalf("api search: status %d", quickSearchResponse.StatusCode)
	}
	var quickSearchBody struct {
		Skills []struct {
			DisplayName string `json:"displayName"`
		} `json:"skills"`
	}
	mustJSONDecode(t, quickSearchResponse, &quickSearchBody)
	if len(quickSearchBody.Skills) != 2 {
		t.Fatalf("api search result len = %d, want 2", len(quickSearchBody.Skills))
	}

	leaderboardResponse := mustRequest(t, anonClient, http.MethodGet, "/leaderboard", nil, nil)
	defer leaderboardResponse.Body.Close()
	if leaderboardResponse.StatusCode != http.StatusOK {
		t.Fatalf("leaderboard: status %d", leaderboardResponse.StatusCode)
	}
	leaderboardBody := mustReadBody(t, leaderboardResponse)
	topStart := strings.Index(leaderboardBody, "热门技能 Top 10")
	activeStart := strings.Index(leaderboardBody, "近期最活跃")
	if topStart == -1 || activeStart == -1 || activeStart <= topStart {
		t.Fatalf("leaderboard sections not found in body: %s", leaderboardBody)
	}
	topSection := leaderboardBody[topStart:activeStart]
	activeSection := leaderboardBody[activeStart:]
	if strings.Index(topSection, "Atlas Agent") == -1 || strings.Index(topSection, "Radar Agent") == -1 {
		t.Fatalf("top leaderboard missing skills: %s", topSection)
	}
	if strings.Index(topSection, "Atlas Agent") > strings.Index(topSection, "Radar Agent") {
		t.Fatalf("top leaderboard order is wrong: %s", topSection)
	}
	if strings.Index(activeSection, "Radar Agent") == -1 || strings.Index(activeSection, "Atlas Agent") == -1 {
		t.Fatalf("active leaderboard missing skills: %s", activeSection)
	}
	if strings.Index(activeSection, "Radar Agent") > strings.Index(activeSection, "Atlas Agent") {
		t.Fatalf("active leaderboard order is wrong: %s", activeSection)
	}
}

func TestIntegrationUploadReviewAndCrudFlow(t *testing.T) {
	resetIntegrationState(t)

	// 管理员要先有，审核链路才跟真实业务一致。
	adminUser, adminTenant := createUserWithTenant(t, "admin@gmail.com", "Platform Admin", true)
	uploaderUser, uploaderTenant := createUserWithTenant(t, "uploader@gmail.com", "Uploader", false)
	adminClient := newAuthenticatedClient(t, adminUser, adminTenant)
	uploaderClient := newAuthenticatedClient(t, uploaderUser, uploaderTenant)

	apiKey := mustCreateAPIKey(t, uploaderClient)
	uploadArchive := buildSkillArchive(t, "Integration Upload Skill", "This upload goes through API review and CRUD checks.")
	uploadResponse := mustMultipartUpload(t, "/api/v1/upload?tenant_id="+fmt.Sprint(uploaderTenant.TenantID), apiKey, uploadArchive)
	if uploadResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResponse.Body)
		t.Fatalf("api upload: status %d body=%s", uploadResponse.StatusCode, string(body))
	}
	var uploadBody struct {
		ID           int64  `json:"id"`
		Slug         string `json:"slug"`
		ReviewStatus string `json:"review_status"`
	}
	mustJSONDecode(t, uploadResponse, &uploadBody)
	if uploadBody.ID == 0 || uploadBody.Slug == "" || uploadBody.ReviewStatus != "pending" {
		t.Fatalf("unexpected upload response: %#v", uploadBody)
	}

	pendingSkill, err := db.GetAdminSkillByID(uploadBody.ID)
	if err != nil {
		t.Fatalf("load pending admin skill: %v", err)
	}
	if pendingSkill == nil || pendingSkill.ReviewStatus != "pending" {
		t.Fatalf("uploaded skill should be pending, got %#v", pendingSkill)
	}

	adminReviewCSRF := mustGetCSRFToken(t, adminClient, fmt.Sprintf("/admin/skill?id=%d", uploadBody.ID))
	approveResponse := mustFormPost(t, adminClient, "/admin/skill/review", url.Values{
		"csrf_token":    {adminReviewCSRF},
		"skill_id":      {fmt.Sprint(uploadBody.ID)},
		"review_status": {"approved"},
		"review_note":   {"looks good"},
	}, nil)
	defer approveResponse.Body.Close()
	if approveResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("approve uploaded skill: status %d", approveResponse.StatusCode)
	}

	approvedSkill, err := db.GetAdminSkillByID(uploadBody.ID)
	if err != nil {
		t.Fatalf("reload approved admin skill: %v", err)
	}
	if approvedSkill == nil || approvedSkill.ReviewStatus != "approved" {
		t.Fatalf("uploaded skill should be approved, got %#v", approvedSkill)
	}

	accountAfterReview := mustRequest(t, uploaderClient, http.MethodGet, "/account", nil, nil)
	defer accountAfterReview.Body.Close()
	if accountAfterReview.StatusCode != http.StatusOK {
		t.Fatalf("account after review: status %d", accountAfterReview.StatusCode)
	}
	if body := mustReadBody(t, accountAfterReview); !strings.Contains(body, "已通过审核") {
		t.Fatalf("review notification missing in account page: %s", body)
	}

	detailResponse := mustRequest(t, newAPIClient(t), http.MethodGet, "/api/v1/skills/"+uploadBody.Slug+"?tenant_id="+fmt.Sprint(uploaderTenant.TenantID), nil, map[string]string{
		"Authorization": "Bearer " + apiKey,
	})
	if detailResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(detailResponse.Body)
		t.Fatalf("api skill detail: status %d body=%s", detailResponse.StatusCode, string(body))
	}
	var detailBody struct {
		Name   string `json:"name"`
		Readme string `json:"readme"`
	}
	mustJSONDecode(t, detailResponse, &detailBody)
	if detailBody.Name != "Integration Upload Skill" || !strings.Contains(detailBody.Readme, "This upload goes through API review") {
		t.Fatalf("unexpected detail payload: %#v", detailBody)
	}

	downloadResponse := mustRequest(t, newAPIClient(t), http.MethodGet, "/api/v1/download/"+fmt.Sprint(uploadBody.ID), nil, map[string]string{
		"Authorization": "Bearer " + apiKey,
	})
	defer downloadResponse.Body.Close()
	if downloadResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(downloadResponse.Body)
		t.Fatalf("api download: status %d body=%s", downloadResponse.StatusCode, string(body))
	}
	downloadedArchive, err := io.ReadAll(downloadResponse.Body)
	if err != nil {
		t.Fatalf("read download archive: %v", err)
	}
	if len(downloadedArchive) == 0 {
		t.Fatal("downloaded archive is empty")
	}

	skillPageCSRF := mustGetCSRFToken(t, uploaderClient, "/skill?slug="+uploadBody.Slug)
	ratingResponse := mustFormPost(t, uploaderClient, "/api/rate", url.Values{
		"csrf_token": {skillPageCSRF},
		"skill_id":   {fmt.Sprint(uploadBody.ID)},
		"score":      {"5"},
	}, nil)
	if ratingResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(ratingResponse.Body)
		t.Fatalf("rate skill: status %d body=%s", ratingResponse.StatusCode, string(body))
	}
	var ratingBody struct {
		Success   bool    `json:"success"`
		AvgRating float64 `json:"avgRating"`
		Count     int     `json:"count"`
		UserScore int     `json:"userScore"`
	}
	mustJSONDecode(t, ratingResponse, &ratingBody)
	if !ratingBody.Success || ratingBody.AvgRating != 5 || ratingBody.Count != 1 || ratingBody.UserScore != 5 {
		t.Fatalf("unexpected rating response: %#v", ratingBody)
	}

	commentCaptcha := mustFetchCaptchaAnswer(t, uploaderClient)
	commentResponse := mustFormPost(t, uploaderClient, "/api/comment", url.Values{
		"csrf_token": {skillPageCSRF},
		"skill_id":   {fmt.Sprint(uploadBody.ID)},
		"slug":       {uploadBody.Slug},
		"content":    {"Integration comment from the uploader"},
		"captcha":    {commentCaptcha},
	}, nil)
	defer commentResponse.Body.Close()
	if commentResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("comment skill: status %d", commentResponse.StatusCode)
	}
	if location := commentResponse.Header.Get("Location"); location != "/skill?slug="+uploadBody.Slug {
		t.Fatalf("comment redirect = %q", location)
	}

	skillPage := mustRequest(t, uploaderClient, http.MethodGet, "/skill?slug="+uploadBody.Slug, nil, nil)
	defer skillPage.Body.Close()
	if skillPage.StatusCode != http.StatusOK {
		t.Fatalf("skill page after comment: status %d", skillPage.StatusCode)
	}
	skillPageBody := mustReadBody(t, skillPage)
	if !strings.Contains(skillPageBody, "Integration comment from the uploader") {
		t.Fatalf("comment not rendered on skill page: %s", skillPageBody)
	}
	if !strings.Contains(skillPageBody, "1 人") {
		t.Fatalf("rating count not rendered on skill page: %s", skillPageBody)
	}

	collectionCreateResponse := mustFormPost(t, uploaderClient, "/collections/create", url.Values{
		"csrf_token":  {skillPageCSRF},
		"name":        {"Integration Collection"},
		"description": {"Collection created in integration test"},
	}, nil)
	defer collectionCreateResponse.Body.Close()
	if collectionCreateResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("create collection: status %d", collectionCreateResponse.StatusCode)
	}
	collections, err := db.ListUserCollections(uploaderUser.ID)
	if err != nil {
		t.Fatalf("list collections after create: %v", err)
	}
	if len(collections) != 1 {
		t.Fatalf("collection count after create = %d, want 1", len(collections))
	}

	collectionAddResponse := mustFormPost(t, uploaderClient, "/collections/add-skill", url.Values{
		"csrf_token":    {skillPageCSRF},
		"collection_id": {fmt.Sprint(collections[0].ID)},
		"skill_id":      {fmt.Sprint(uploadBody.ID)},
	}, map[string]string{"Referer": "/skill?slug=" + uploadBody.Slug})
	defer collectionAddResponse.Body.Close()
	if collectionAddResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("add skill to collection: status %d", collectionAddResponse.StatusCode)
	}
	collectionDetail, err := db.GetCollection(collections[0].ID)
	if err != nil {
		t.Fatalf("get collection after add: %v", err)
	}
	if collectionDetail == nil || collectionDetail.ItemCount != 1 {
		t.Fatalf("collection item count = %#v, want 1", collectionDetail)
	}

	collectionDeleteResponse := mustFormPost(t, uploaderClient, "/collections/delete", url.Values{
		"csrf_token":    {skillPageCSRF},
		"collection_id": {fmt.Sprint(collections[0].ID)},
	}, nil)
	defer collectionDeleteResponse.Body.Close()
	if collectionDeleteResponse.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete collection: status %d", collectionDeleteResponse.StatusCode)
	}
	collections, err = db.ListUserCollections(uploaderUser.ID)
	if err != nil {
		t.Fatalf("list collections after delete: %v", err)
	}
	if len(collections) != 0 {
		t.Fatalf("collection count after delete = %d, want 0", len(collections))
	}
}

func newAPIClient(t *testing.T) *http.Client {
	t.Helper()
	return newBrowserClient(t)
}

func buildSkillArchive(t *testing.T, name, description string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create("SKILL.md")
	if err != nil {
		t.Fatalf("create skill.md entry: %v", err)
	}
	content := fmt.Sprintf("# %s\n\n%s\n", name, description)
	if _, err := io.WriteString(file, content); err != nil {
		t.Fatalf("write skill.md content: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip archive: %v", err)
	}
	return buffer.Bytes()
}

func mustMultipartUpload(t *testing.T, path, apiKey string, archive []byte) *http.Response {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("zipfile", "integration-skill.zip")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := file.Write(archive); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return mustRequest(t, newAPIClient(t), http.MethodPost, path, &body, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  writer.FormDataContentType(),
	})
}
