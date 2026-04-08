package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

var ClawHubBaseURL = "https://wry-manatee-359.convex.site/api/search"

var SyncKeywords = []string{
	"browser", "agent", "github", "weather", "slack", "email", "file",
	"search", "web", "code", "deploy", "docker", "database", "api",
	"automation", "ai", "ml", "machine learning", "twitter", "discord",
	"calendar", "notes", "knowledge", "memory", "shell", "terminal",
	"monitoring", "analytics", "payment", "stripe", "sms", "notification",
	"image", "video", "audio", "pdf", "scraping",
}

func SyncAllActiveTenants() {
	tenantIDs, err := ListAutoSyncTenantIDs()
	if err != nil {
		log.Printf("读取自动同步租户失败: %v", err)
		return
	}

	for _, tenantID := range tenantIDs {
		if !StartTenantSync(tenantID) {
			log.Printf("租户 %d 已在同步中，跳过本轮自动同步", tenantID)
			continue
		}
		if err := SyncFromClawHub(tenantID); err != nil {
			log.Printf("租户 %d 自动同步失败: %v", tenantID, err)
		}
		FinishTenantSync(tenantID)
	}
}

func SyncFromClawHub(tenantID int64) error {
	log.Printf("Starting sync from ClawHub for tenant %d", tenantID)
	totalSynced := 0
	var syncErrors []string

	for _, keyword := range SyncKeywords {
		results, err := fetchFromClawHub(keyword)
		if err != nil {
			log.Printf("Error fetching '%s': %v", keyword, err)
			syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", keyword, err))
			continue
		}

		synced, err := saveSkills(tenantID, results)
		if err != nil {
			log.Printf("Error saving '%s': %v", keyword, err)
			syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", keyword, err))
			continue
		}
		totalSynced += synced
		time.Sleep(100 * time.Millisecond)
	}

	status := "success"
	message := ""
	if len(syncErrors) > 0 {
		status = "failed"
		message = strings.Join(syncErrors, "; ")
		if len(message) > 500 {
			message = message[:500]
		}
	}

	if err := logSync(tenantID, totalSynced, strings.Join(SyncKeywords, ","), status, message); err != nil {
		return err
	}
	if len(syncErrors) > 0 {
		return fmt.Errorf("sync completed with %d errors", len(syncErrors))
	}
	log.Printf("Sync completed for tenant %d. Total skills synced: %d", tenantID, totalSynced)
	return nil
}

func fetchFromClawHub(keyword string) ([]models.Skill, error) {
	url := fmt.Sprintf("%s?q=%s&limit=20", ClawHubBaseURL, url.QueryEscape(keyword))

	resp, err := fetchHTTP(url)
	if err != nil {
		return nil, err
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	skills := make([]models.Skill, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		skill := models.Skill{
			Slug:        r.Slug,
			DisplayName: r.DisplayName,
			Summary:     r.Summary,
			Score:       r.Score,
			UpdatedAt:   time.Unix(r.UpdatedAt/1000, 0),
			Version:     r.Version,
			Categories:  categorizeSkill(r.DisplayName, r.Summary),
			Source:      "clawhub",
			ClawHubURL:  fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", r.Slug),
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

func fetchHTTP(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Skills-Hub/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func saveSkills(tenantID int64, skills []models.Skill) (int, error) {
	if len(skills) == 0 {
		return 0, nil
	}

	tx, err := GetDB().Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO skills (tenant_id, slug, display_name, summary, score, source_updated_at, version, categories, author, source, review_status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'approved', CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, slug) DO UPDATE SET
			display_name = excluded.display_name,
			summary = excluded.summary,
			score = excluded.score,
			source_updated_at = excluded.source_updated_at,
			version = excluded.version,
			categories = excluded.categories,
			author = excluded.author,
			source = excluded.source,
			review_status = CASE WHEN skills.source = 'upload' THEN skills.review_status ELSE 'approved' END,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, skill := range skills {
		// 第三方同步回来的字段也按不可信输入处理。
		displayName := security.EscapePlainText(skill.DisplayName)
		summary := security.EscapePlainText(skill.Summary)
		version := security.EscapePlainText(skill.Version)
		categories := security.EscapePlainText(skill.Categories)
		author := security.EscapePlainText("ClawHub")
		source := security.EscapePlainText(skill.Source)

		if _, err := stmt.Exec(tenantID, skill.Slug, displayName, summary, skill.Score, skill.UpdatedAt.Unix(), version, categories, author, source); err == nil {
			count++
		} else {
			log.Printf("保存 skill %s 失败: %v", skill.Slug, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return count, err
	}

	// 同步完成后重建 FTS 索引（批量操作比逐条更快）
	if count > 0 {
		rebuildFTSIndex()
	}

	return count, nil
}

func logSync(tenantID int64, count int, keywords, status, message string) error {
	_, err := GetDB().Exec(`
		INSERT INTO sync_log (tenant_id, count, keywords, status, message)
		VALUES (?, ?, ?, ?, ?)
	`, tenantID, count, keywords, status, message)
	return err
}

// CategorizeByText 根据文本内容自动分类，上传时也会用到
func CategorizeByText(name, summary string) string {
	return categorizeSkill(name, summary)
}

func categorizeSkill(name, summary string) string {
	text := strings.ToLower(name + " " + summary)

	categories := []struct {
		keywords []string
		category string
	}{
		{[]string{"browser", "chrome", "web", "scrape"}, "浏览器自动化"},
		{[]string{"ai", "ml", "machine learning", "gpt", "llm", "openai", "claude"}, "AI/ML"},
		{[]string{"devops", "docker", "deploy", "kubernetes", "k8s", "ci/cd"}, "运维/DevOps"},
		{[]string{"code", "git", "github", "repo", "programming", "developer"}, "开发工具"},
		{[]string{"slack", "discord", "chat", "message", "telegram", "messaging"}, "聊天/通讯"},
		{[]string{"file", "pdf", "document", "image", "video", "audio", "media"}, "文件处理"},
		{[]string{"api", "rest", "graphql", "http", "endpoint"}, "API集成"},
		{[]string{"database", "db", "sql", "postgres", "mysql", "mongodb"}, "数据库"},
		{[]string{"email", "mail", "smtp", "gmail"}, "邮件"},
		{[]string{"calendar", "schedule", "event", "meeting"}, "日历"},
		{[]string{"weather", "forecast", "meteorological"}, "天气"},
		{[]string{"twitter", "social", "instagram", "facebook", "linkedin"}, "社交媒体"},
		{[]string{"monitor", "alert", "metrics", "grafana", "prometheus"}, "监控"},
		{[]string{"payment", "stripe", "billing", "invoice"}, "支付"},
		{[]string{"voice", "speech", "tts", "stt", "audio"}, "语音处理"},
		{[]string{"search", "find", "discover", "explore"}, "搜索发现"},
	}

	var matched []string
	for _, item := range categories {
		for _, keyword := range item.keywords {
			if strings.Contains(text, keyword) {
				if !containsStr(matched, item.category) {
					matched = append(matched, item.category)
				}
				break
			}
		}
	}

	if len(matched) == 0 {
		matched = append(matched, "其他")
	}

	return strings.Join(matched, ",")
}

func containsStr(slice []string, item string) bool {
	for _, current := range slice {
		if current == item {
			return true
		}
	}
	return false
}

func GetAllSkills(tenantID int64) ([]models.Skill, error) {
	return querySkills(`
		SELECT id, tenant_id, slug, display_name, summary, content, score, source_updated_at, version, categories, source
		FROM skills WHERE tenant_id = ? AND (source != 'upload' OR review_status = 'approved')
		ORDER BY score DESC, display_name ASC
	`, tenantID)
}

func SearchSkills(tenantID int64, query string) ([]models.Skill, error) {
	return GetFilteredSkills(tenantID, query, "", "")
}

func GetSkillBySlug(tenantID int64, slug string) (*models.Skill, error) {
	return getSkillBySlug(tenantID, slug, true)
}

func GetSkillBySlugAnyStatus(tenantID int64, slug string) (*models.Skill, error) {
	return getSkillBySlug(tenantID, slug, false)
}

func getSkillBySlug(tenantID int64, slug string, onlyApprovedUploads bool) (*models.Skill, error) {
	query := `
		SELECT id, tenant_id, slug, display_name, summary, content, score, source_updated_at, version, categories, source
		FROM skills WHERE tenant_id = ? AND slug = ?
	`
	if onlyApprovedUploads {
		query += ` AND (source != 'upload' OR review_status = 'approved')`
	}

	row := GetDB().QueryRow(query, tenantID, slug)

	var skill models.Skill
	var updatedAt int64
	if err := row.Scan(&skill.ID, &skill.TenantID, &skill.Slug, &skill.DisplayName, &skill.Summary, &skill.Content, &skill.Score, &updatedAt, &skill.Version, &skill.Categories, &skill.Source); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	skill.UpdatedAt = time.Unix(updatedAt, 0)
	skill.ClawHubURL = fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", skill.Slug)
	decodeSkillForDisplay(&skill)
	return &skill, nil
}

// sortBy: "score"(默认), "rating", "latest"
func GetFilteredSkills(tenantID int64, query, category, sortBy string) ([]models.Skill, error) {
	statement, args := buildFilteredSkillsQuery(tenantID, query, category)
	statement += ` GROUP BY s.id`
	statement += skillOrderClause(sortBy)

	return querySkillsWithRating(statement, args...)
}

// GetFilteredSkillsPage 先查总数，再只拿当前页数据。
func GetFilteredSkillsPage(tenantID int64, query, category, sortBy string, page, perPage int) ([]models.Skill, int, int, error) {
	countStatement, countArgs := buildFilteredSkillsCountQuery(tenantID, query, category)
	total, err := countSkills(countStatement, countArgs...)
	if err != nil {
		return nil, 0, 1, err
	}

	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 12
	}

	if total == 0 {
		return []models.Skill{}, 0, 1, nil
	}

	totalPages := (total + perPage - 1) / perPage
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * perPage
	statement, args := buildFilteredSkillsQuery(tenantID, query, category)
	statement += ` GROUP BY s.id`
	statement += skillOrderClause(sortBy)
	statement += ` LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	skills, err := querySkillsWithRating(statement, args...)
	if err != nil {
		return nil, 0, 1, err
	}

	return skills, total, page, nil
}

func buildFilteredSkillsQuery(tenantID int64, query, category string) (string, []interface{}) {
	// 基础查询，左联评分表拿用户评分均值。
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
	`
	whereClause, args := buildFilteredSkillsWhereClause(tenantID, query, category)
	return statement + whereClause, args
}

func buildFilteredSkillsCountQuery(tenantID int64, query, category string) (string, []interface{}) {
	statement := `SELECT COUNT(*) FROM skills s`
	whereClause, args := buildFilteredSkillsWhereClause(tenantID, query, category)
	return statement + whereClause, args
}

// 高级搜索参数
type AdvancedSearchParams struct {
	Query     string
	Category  string
	MinRating float64 // 最低用户评分
	DateRange string  // "7d", "30d", "90d", ""
	Author    string
	Source    string // "clawhub", "upload", ""
}

func buildFilteredSkillsWhereClause(tenantID int64, query, category string) (string, []interface{}) {
	return buildAdvancedWhereClause(tenantID, AdvancedSearchParams{Query: query, Category: category})
}

func buildAdvancedWhereClause(tenantID int64, params AdvancedSearchParams) (string, []interface{}) {
	statement := `
		WHERE s.tenant_id = ? AND (s.source != 'upload' OR s.review_status = 'approved')
	`
	args := []interface{}{tenantID}

	// 全文搜索
	if params.Query != "" {
		ftsIDs := SearchSkillIDsByFTS(params.Query)
		if ftsIDs != nil && len(ftsIDs) > 0 {
			placeholders := make([]string, len(ftsIDs))
			for i, id := range ftsIDs {
				placeholders[i] = "?"
				args = append(args, id)
			}
			statement += ` AND s.id IN (` + strings.Join(placeholders, ",") + `)`
		} else {
			pattern := "%" + params.Query + "%"
			statement += ` AND (s.display_name LIKE ? OR s.summary LIKE ? OR s.categories LIKE ?)`
			args = append(args, pattern, pattern, pattern)
		}
	}

	// 分类筛选
	if params.Category != "" {
		statement += ` AND s.categories LIKE ?`
		args = append(args, "%"+params.Category+"%")
	}

	// 来源筛选
	if params.Source == "clawhub" {
		statement += ` AND s.source != 'upload'`
	} else if params.Source == "upload" {
		statement += ` AND s.source = 'upload'`
	}

	// 作者筛选
	if params.Author != "" {
		statement += ` AND s.author LIKE ?`
		args = append(args, "%"+params.Author+"%")
	}

	// 日期范围
	switch params.DateRange {
	case "7d":
		statement += ` AND s.created_at >= datetime('now', '-7 days')`
	case "30d":
		statement += ` AND s.created_at >= datetime('now', '-30 days')`
	case "90d":
		statement += ` AND s.created_at >= datetime('now', '-90 days')`
	}

	return statement, args
}

// 带高级参数的分页搜索
func GetFilteredSkillsPageAdvanced(tenantID int64, params AdvancedSearchParams, sortBy string, page, perPage int) ([]models.Skill, int, int, error) {
	countStatement := `SELECT COUNT(*) FROM skills s`
	whereClause, args := buildAdvancedWhereClause(tenantID, params)
	total, err := countSkills(countStatement+whereClause, args...)
	if err != nil {
		return nil, 0, 1, err
	}

	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 12
	}
	if total == 0 {
		return []models.Skill{}, 0, 1, nil
	}

	totalPages := (total + perPage - 1) / perPage
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * perPage
	baseQuery := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
		       s.source_updated_at, s.version, s.categories, s.source,
		       COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
	`
	whereClause2, args2 := buildAdvancedWhereClause(tenantID, params)
	statement := baseQuery + whereClause2 + ` GROUP BY s.id`

	// 最低评分筛选放在 HAVING（因为 avg_rating 是聚合结果）
	if params.MinRating > 0 {
		statement += ` HAVING avg_rating >= ?`
		args2 = append(args2, params.MinRating)
	}

	statement += skillOrderClause(sortBy)
	statement += ` LIMIT ? OFFSET ?`
	args2 = append(args2, perPage, offset)

	skills, err := querySkillsWithRating(statement, args2...)
	if err != nil {
		return nil, 0, 1, err
	}

	return skills, total, page, nil
}

func skillOrderClause(sortBy string) string {
	// 排序条件单独拆出来，列表查询和分页查询共用一套。
	switch sortBy {
	case "rating":
		return ` ORDER BY avg_rating DESC, s.display_name ASC`
	case "latest":
		return ` ORDER BY s.created_at DESC, s.display_name ASC`
	case "relevance":
		// FTS 搜索时按相关度排序（ID 在 FTS 结果中的顺序就是相关度顺序）
		return ` ORDER BY s.score DESC, s.display_name ASC`
	default:
		return ` ORDER BY s.score DESC, s.display_name ASC`
	}
}

func countSkills(statement string, args ...interface{}) (int, error) {
	var total int
	if err := GetDB().QueryRow(statement, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// 带评分统计的查询
func querySkillsWithRating(statement string, args ...interface{}) ([]models.Skill, error) {
	rows, err := GetDB().Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []models.Skill
	for rows.Next() {
		var skill models.Skill
		var updatedAt int64
		if err := rows.Scan(&skill.ID, &skill.TenantID, &skill.Slug, &skill.DisplayName, &skill.Summary, &skill.Content, &skill.Score, &updatedAt, &skill.Version, &skill.Categories, &skill.Source, &skill.AvgRating, &skill.RatingCount); err != nil {
			return nil, err
		}
		skill.UpdatedAt = time.Unix(updatedAt, 0)
		skill.ClawHubURL = fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", skill.Slug)
		decodeSkillForDisplay(&skill)
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

func querySkills(statement string, args ...interface{}) ([]models.Skill, error) {
	rows, err := GetDB().Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []models.Skill
	for rows.Next() {
		var skill models.Skill
		var updatedAt int64
		if err := rows.Scan(&skill.ID, &skill.TenantID, &skill.Slug, &skill.DisplayName, &skill.Summary, &skill.Content, &skill.Score, &updatedAt, &skill.Version, &skill.Categories, &skill.Source); err != nil {
			return nil, err
		}
		skill.UpdatedAt = time.Unix(updatedAt, 0)
		skill.ClawHubURL = fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", skill.Slug)
		decodeSkillForDisplay(&skill)
		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

func GetCategories(tenantID int64) ([]string, error) {
	rows, err := GetDB().Query(`SELECT DISTINCT categories FROM skills WHERE tenant_id = ? AND categories != '' AND (source != 'upload' OR review_status = 'approved')`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := make(map[string]bool)
	for rows.Next() {
		var categories string
		if err := rows.Scan(&categories); err != nil {
			return nil, err
		}
		for _, category := range strings.Split(categories, ",") {
			category = strings.TrimSpace(security.DecodeStoredText(category))
			if category != "" {
				set[category] = true
			}
		}
	}

	var result []string
	for category := range set {
		result = append(result, category)
	}
	sort.Strings(result)
	return result, rows.Err()
}

func GetLatestSyncLog(tenantID int64) (*time.Time, int, string, string, error) {
	row := GetDB().QueryRow(`
		SELECT synced_at, count, status, message
		FROM sync_log WHERE tenant_id = ?
		ORDER BY id DESC LIMIT 1
	`, tenantID)

	var syncedAt time.Time
	var count int
	var status string
	var message string
	if err := row.Scan(&syncedAt, &count, &status, &message); err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, "", "", nil
		}
		return nil, 0, "", "", err
	}
	return &syncedAt, count, status, message, nil
}

var (
	syncMutex sync.Mutex
	syncState = make(map[int64]bool)
)

func StartTenantSync(tenantID int64) bool {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	if syncState[tenantID] {
		return false
	}
	// 原子置位，手动同步和自动同步就不会抢到同一个租户了。
	syncState[tenantID] = true
	return true
}

func FinishTenantSync(tenantID int64) {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	delete(syncState, tenantID)
}

func IsSyncing(tenantID int64) bool {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	return syncState[tenantID]
}

func SetSyncing(tenantID int64, status bool) {
	if status {
		StartTenantSync(tenantID)
		return
	}
	FinishTenantSync(tenantID)
}
