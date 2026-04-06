package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		if err := SyncFromClawHub(tenantID); err != nil {
			log.Printf("租户 %d 自动同步失败: %v", tenantID, err)
		}
	}
}

func SyncFromClawHub(tenantID int64) error {
	log.Printf("Starting sync from ClawHub for tenant %d", tenantID)
	totalSynced := 0

	for _, keyword := range SyncKeywords {
		results, err := fetchFromClawHub(keyword)
		if err != nil {
			log.Printf("Error fetching '%s': %v", keyword, err)
			continue
		}

		synced := saveSkills(tenantID, results)
		totalSynced += synced
		time.Sleep(100 * time.Millisecond)
	}

	if err := logSync(tenantID, totalSynced, strings.Join(SyncKeywords, ","), "success", ""); err != nil {
		return err
	}
	log.Printf("Sync completed for tenant %d. Total skills synced: %d", tenantID, totalSynced)
	return nil
}

func fetchFromClawHub(keyword string) ([]models.Skill, error) {
	url := fmt.Sprintf("%s?q=%s&limit=20", ClawHubBaseURL, keyword)

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

func saveSkills(tenantID int64, skills []models.Skill) int {
	if len(skills) == 0 {
		return 0
	}

	tx, err := GetDB().Begin()
	if err != nil {
		return 0
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO skills (tenant_id, slug, display_name, summary, score, source_updated_at, version, categories, author, source, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, slug) DO UPDATE SET
			display_name = excluded.display_name,
			summary = excluded.summary,
			score = excluded.score,
			source_updated_at = excluded.source_updated_at,
			version = excluded.version,
			categories = excluded.categories,
			author = excluded.author,
			source = excluded.source,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return 0
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
		}
	}

	_ = tx.Commit()
	return count
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
		FROM skills WHERE tenant_id = ?
		ORDER BY score DESC, display_name ASC
	`, tenantID)
}

func SearchSkills(tenantID int64, query string) ([]models.Skill, error) {
	return GetFilteredSkills(tenantID, query, "", "")
}

func GetSkillBySlug(tenantID int64, slug string) (*models.Skill, error) {
	row := GetDB().QueryRow(`
		SELECT id, tenant_id, slug, display_name, summary, content, score, source_updated_at, version, categories, source
		FROM skills WHERE tenant_id = ? AND slug = ?
	`, tenantID, slug)

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
	// 基础查询，左联评分表拿用户评分均值
	statement := `
		SELECT s.id, s.tenant_id, s.slug, s.display_name, s.summary, s.content, s.score,
			   s.source_updated_at, s.version, s.categories, s.source,
			   COALESCE(AVG(r.score), 0) as avg_rating, COUNT(r.id) as rating_count
		FROM skills s
		LEFT JOIN skill_ratings r ON r.skill_id = s.id AND r.tenant_id = s.tenant_id
		WHERE s.tenant_id = ?
	`
	args := []interface{}{tenantID}

	if query != "" {
		pattern := "%" + query + "%"
		statement += ` AND (s.display_name LIKE ? OR s.summary LIKE ? OR s.categories LIKE ?)`
		args = append(args, pattern, pattern, pattern)
	}
	if category != "" {
		statement += ` AND s.categories LIKE ?`
		args = append(args, "%"+category+"%")
	}

	statement += ` GROUP BY s.id`

	// 排序方式
	switch sortBy {
	case "rating":
		statement += ` ORDER BY avg_rating DESC, s.display_name ASC`
	case "latest":
		statement += ` ORDER BY s.created_at DESC, s.display_name ASC`
	default:
		statement += ` ORDER BY s.score DESC, s.display_name ASC`
	}

	return querySkillsWithRating(statement, args...)
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
	rows, err := GetDB().Query(`SELECT DISTINCT categories FROM skills WHERE tenant_id = ? AND categories != ''`, tenantID)
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

func IsSyncing(tenantID int64) bool {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	return syncState[tenantID]
}

func SetSyncing(tenantID int64, status bool) {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	if status {
		syncState[tenantID] = true
		return
	}
	delete(syncState, tenantID)
}
