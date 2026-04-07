package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skills-hub/db"
	"skills-hub/security"
)

type apiSearchSkill struct {
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Version       string  `json:"version"`
	Author        string  `json:"author"`
	RatingAvg     float64 `json:"rating_avg"`
	RatingCount   int     `json:"rating_count"`
	DownloadCount int     `json:"download_count"`
	CreatedAt     string  `json:"created_at"`
}

type apiSkillDetail struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Author        string   `json:"author"`
	Readme        string   `json:"readme"`
	RatingAvg     float64  `json:"rating_avg"`
	RatingCount   int      `json:"rating_count"`
	DownloadCount int      `json:"download_count"`
	CreatedAt     string   `json:"created_at"`
	Keywords      []string `json:"keywords"`
}

type apiCategoryItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func APIV1SearchHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveAPITenantScope(w, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	skills, err := db.SearchSkillsForAPI(strings.TrimSpace(r.URL.Query().Get("q")), strings.TrimSpace(r.URL.Query().Get("sort")), tenantID)
	if err != nil {
		log.Printf("api search failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "搜索失败")
		return
	}

	result := make([]apiSearchSkill, 0, len(skills))
	for _, skill := range skills {
		result = append(result, apiSearchSkill{
			Slug:          skill.Slug,
			Name:          skill.Name,
			Description:   skill.Description,
			Version:       skill.Version,
			Author:        skill.Author,
			RatingAvg:     skill.RatingAvg,
			RatingCount:   skill.RatingCount,
			DownloadCount: skill.DownloadCount,
			CreatedAt:     skill.CreatedAt.UTC().Format(timeLayout),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func APIV1SkillDetailHandler(w http.ResponseWriter, r *http.Request) {
	slug, err := apiSlugFromPath(r.URL.Path, "/api/v1/skills/")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "skill slug 不正确")
		return
	}
	tenantID, err := resolveAPITenantScope(w, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	skill, err := db.GetSkillBySlugForAPI(slug, tenantID)
	if err != nil {
		log.Printf("api skill detail failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "读取 skill 失败")
		return
	}
	if skill == nil {
		writeAPIError(w, http.StatusNotFound, "skill 不存在")
		return
	}

	readme, err := security.RenderSkillMarkdown(skillMarkdownSource(*skill))
	if err != nil {
		log.Printf("api skill render failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "渲染 skill 失败")
		return
	}

	writeJSON(w, http.StatusOK, apiSkillDetail{
		Slug:          skill.Slug,
		Name:          skill.Name,
		Description:   skill.Description,
		Version:       skill.Version,
		Author:        skill.Author,
		Readme:        string(readme),
		RatingAvg:     skill.RatingAvg,
		RatingCount:   skill.RatingCount,
		DownloadCount: skill.DownloadCount,
		CreatedAt:     skill.CreatedAt.UTC().Format(timeLayout),
		Keywords:      buildSkillKeywords(*skill),
	})
}

func APIV1DownloadHandler(w http.ResponseWriter, r *http.Request) {
	slug, err := apiSlugFromPath(r.URL.Path, "/api/v1/download/")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "skill slug 不正确")
		return
	}
	tenantID, err := resolveAPITenantScope(w, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	skill, err := db.GetSkillBySlugForAPI(slug, tenantID)
	if err != nil {
		log.Printf("api download failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "读取 skill 失败")
		return
	}
	if skill == nil {
		writeAPIError(w, http.StatusNotFound, "skill 不存在")
		return
	}

	fileName := skill.Slug + ".zip"

	if skill.Source == "upload" {
		zipPath := filepath.Join("./uploads", fmt.Sprintf("%d", skill.TenantID), skill.Slug+".zip")
		data, err := os.ReadFile(zipPath)
		if err != nil {
			log.Printf("read uploaded zip failed: %v", err)
			writeAPIError(w, http.StatusNotFound, "原始 zip 不存在")
			return
		}
		if err := db.IncrementSkillDownloadCount(skill.ID); err != nil {
			log.Printf("increment download count failed: %v", err)
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
		_, _ = w.Write(data)
		return
	}

	buf, err := buildSkillZIP(*skill)
	if err != nil {
		log.Printf("build sync zip failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "打包 zip 失败")
		return
	}
	if err := db.IncrementSkillDownloadCount(skill.ID); err != nil {
		log.Printf("increment download count failed: %v", err)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	_, _ = w.Write(buf.Bytes())
}

func APIV1CategoriesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveAPITenantScope(w, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	counts, err := db.ListCategoryCountsForAPI(tenantID)
	if err != nil {
		log.Printf("api categories failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "读取分类失败")
		return
	}

	items := make([]apiCategoryItem, 0, len(counts))
	for name, count := range counts {
		items = append(items, apiCategoryItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})

	writeJSON(w, http.StatusOK, items)
}

func APIV1StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetPlatformStatsForAPI()
	if err != nil {
		log.Printf("api stats failed: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "读取统计失败")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

func parseTenantIDQuery(r *http.Request) (*int64, error) {
	value := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if value == "" {
		return nil, nil
	}
	parsed, err := parseInt64(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func resolveAPITenantScope(w http.ResponseWriter, r *http.Request) (*int64, error) {
	tenantID, err := parseTenantIDQuery(r)
	if err != nil {
		return nil, fmt.Errorf("tenant_id 参数不正确")
	}
	if tenantID == nil || *tenantID == 0 {
		return nil, nil
	}

	_, ok, err := requireTenantMembership(w, r, *tenantID)
	if err != nil {
		return nil, fmt.Errorf("租户校验失败")
	}
	if !ok {
		return nil, fmt.Errorf("无权访问该租户数据")
	}

	return tenantID, nil
}

func writeAPITenantScopeError(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	if err != nil && err.Error() == "tenant_id 参数不正确" {
		status = http.StatusBadRequest
	}
	writeAPIError(w, status, err.Error())
}

func apiSlugFromPath(pathValue, prefix string) (string, error) {
	slug := strings.TrimPrefix(pathValue, prefix)
	slug = strings.TrimSpace(slug)
	if slug == "" || strings.Contains(slug, "/") {
		return "", fmt.Errorf("invalid slug")
	}
	decoded, err := url.PathUnescape(slug)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func skillMarkdownSource(skill db.APISkillRecord) string {
	if strings.TrimSpace(skill.Content) != "" {
		return skill.Content
	}

	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(skill.Name)
	builder.WriteString("\n\n")
	if skill.Description != "" {
		builder.WriteString(skill.Description)
		builder.WriteString("\n\n")
	}
	if skill.Version != "" {
		builder.WriteString("- Version: `")
		builder.WriteString(skill.Version)
		builder.WriteString("`\n")
	}
	if skill.Author != "" {
		builder.WriteString("- Author: ")
		builder.WriteString(skill.Author)
		builder.WriteString("\n")
	}
	if skill.Categories != "" {
		builder.WriteString("- Categories: ")
		builder.WriteString(skill.Categories)
		builder.WriteString("\n")
	}
	return builder.String()
}

func buildSkillKeywords(skill db.APISkillRecord) []string {
	meta := parseSkillMD(skill.Content)
	raw := meta.Keywords
	if raw == "" {
		raw = meta.Categories
	}
	if raw == "" {
		raw = skill.Categories
	}

	set := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}

	keywords := make([]string, 0, len(set))
	for item := range set {
		keywords = append(keywords, item)
	}
	sort.Strings(keywords)
	return keywords
}

func buildSkillZIP(skill db.APISkillRecord) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buf)

	entry, err := zipWriter.Create(skill.Slug + "/SKILL.md")
	if err != nil {
		return nil, err
	}
	if _, err := entry.Write([]byte(skillMarkdownSource(skill))); err != nil {
		return nil, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}
