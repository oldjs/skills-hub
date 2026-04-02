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

	_ "modernc.org/sqlite"
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

func SyncFromClawHub() error {
	log.Println("Starting sync from ClawHub...")
	totalSynced := 0

	for _, keyword := range SyncKeywords {
		results, err := fetchFromClawHub(keyword)
		if err != nil {
			log.Printf("Error fetching '%s': %v", keyword, err)
			continue
		}

		synced := saveSkills(results)
		totalSynced += synced
		log.Printf("Synced %d skills for keyword '%s'", synced, keyword)
		time.Sleep(100 * time.Millisecond)
	}

	logSync(totalSynced, strings.Join(SyncKeywords, ","))
	log.Printf("Sync completed. Total skills synced: %d", totalSynced)
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

func saveSkills(skills []models.Skill) int {
	if len(skills) == 0 {
		return 0
	}

	db := GetDB()
	tx, err := db.Begin()
	if err != nil {
		return 0
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO skills (slug, display_name, summary, score, updated_at, version, categories)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0
	}
	defer stmt.Close()

	count := 0
	for _, s := range skills {
		_, err := stmt.Exec(s.Slug, s.DisplayName, s.Summary, s.Score, s.UpdatedAt.Unix(), s.Version, s.Categories)
		if err == nil {
			count++
		}
	}

	tx.Commit()
	return count
}

func logSync(count int, keywords string) {
	db := GetDB()
	db.Exec("INSERT INTO sync_log (count, keywords) VALUES (?, ?)", count, keywords)
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
	for _, c := range categories {
		for _, kw := range c.keywords {
			if strings.Contains(text, kw) {
				if !containsStr(matched, c.category) {
					matched = append(matched, c.category)
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
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func GetAllSkills() ([]models.Skill, error) {
	return querySkills(`
		SELECT id, slug, display_name, summary, score, updated_at, version, categories
		FROM skills ORDER BY score DESC, display_name ASC
	`)
}

func SearchSkills(query string) ([]models.Skill, error) {
	return GetFilteredSkills(query, "")
}

func GetSkillBySlug(slug string) (*models.Skill, error) {
	db := GetDB()
	row := db.QueryRow(`
		SELECT id, slug, display_name, summary, score, updated_at, version, categories
		FROM skills WHERE slug = ?
	`, slug)

	var s models.Skill
	var updatedAt int64
	err := row.Scan(&s.ID, &s.Slug, &s.DisplayName, &s.Summary, &s.Score, &updatedAt, &s.Version, &s.Categories)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.UpdatedAt = time.Unix(updatedAt, 0)
	s.ClawHubURL = fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", s.Slug)

	return &s, nil
}

func GetSkillsByCategory(category string) ([]models.Skill, error) {
	return GetFilteredSkills("", category)
}

func GetFilteredSkills(query, category string) ([]models.Skill, error) {
	statement := `
		SELECT id, slug, display_name, summary, score, updated_at, version, categories
		FROM skills
	`

	var clauses []string
	var args []interface{}
	if query != "" {
		pattern := "%" + query + "%"
		clauses = append(clauses, "(display_name LIKE ? OR summary LIKE ? OR categories LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	if category != "" {
		clauses = append(clauses, "categories LIKE ?")
		args = append(args, "%"+category+"%")
	}
	if len(clauses) > 0 {
		statement += " WHERE " + strings.Join(clauses, " AND ")
	}
	statement += " ORDER BY score DESC, display_name ASC"

	return querySkills(statement, args...)
}

func querySkills(statement string, args ...interface{}) ([]models.Skill, error) {
	db := GetDB()
	rows, err := db.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []models.Skill
	for rows.Next() {
		var s models.Skill
		var updatedAt int64
		err := rows.Scan(&s.ID, &s.Slug, &s.DisplayName, &s.Summary, &s.Score, &updatedAt, &s.Version, &s.Categories)
		if err != nil {
			continue
		}
		s.UpdatedAt = time.Unix(updatedAt, 0)
		s.ClawHubURL = fmt.Sprintf("https://clawhub.ai/skills?focus=search&q=%s", s.Slug)
		skills = append(skills, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return skills, nil
}

func GetCategories() ([]string, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT DISTINCT categories FROM skills WHERE categories IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categorySet := make(map[string]bool)
	for rows.Next() {
		var categories string
		if err := rows.Scan(&categories); err != nil {
			continue
		}
		for _, c := range strings.Split(categories, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				categorySet[c] = true
			}
		}
	}

	var result []string
	for c := range categorySet {
		result = append(result, c)
	}
	sort.Strings(result)
	return result, nil
}

var (
	syncMutex sync.Mutex
	isSyncing bool
)

func IsSyncing() bool {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	return isSyncing
}

func SetSyncing(status bool) {
	syncMutex.Lock()
	isSyncing = status
	syncMutex.Unlock()
}
