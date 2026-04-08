package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	ID            int64   `json:"id"`
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Version       string  `json:"version"`
	Author        string  `json:"author"`
	Rating        float64 `json:"rating"`
	DownloadCount int     `json:"download_count"`
	Category      string  `json:"category"`
}

type apiSkillDetail struct {
	ID            int64    `json:"id"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Author        string   `json:"author"`
	Readme        string   `json:"readme"`
	Rating        float64  `json:"rating"`
	RatingCount   int      `json:"rating_count"`
	DownloadCount int      `json:"download_count"`
	Category      string   `json:"category"`
	Keywords      []string `json:"keywords"`
}

type apiCategoryItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func APIV1SearchHandler(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := requireAPIKeyAuth(w, r)
	if !ok {
		return
	}

	tenantID, err := resolveAPITenantScopeForUser(authCtx.User, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	perPage := parsePositiveInt(r.URL.Query().Get("per_page"), 20)
	if perPage > 100 {
		perPage = 100
	}

	skills, err := db.SearchSkillsForAPI(query, category, sortBy, tenantID)
	if err != nil {
		slog.Error("api search failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "搜索失败")
		return
	}

	total := len(skills)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	result := make([]apiSearchSkill, 0, end-start)
	for _, skill := range skills[start:end] {
		result = append(result, apiSearchSkill{
			ID:            skill.ID,
			Slug:          skill.Slug,
			Name:          skill.Name,
			Description:   skill.Description,
			Version:       skill.Version,
			Author:        skill.Author,
			Rating:        skill.RatingAvg,
			DownloadCount: skill.DownloadCount,
			Category:      skill.Categories,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skills":   result,
		"page":     page,
		"per_page": perPage,
		"total":    total,
	})
}

func APIV1SkillDetailHandler(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := requireAPIKeyAuth(w, r)
	if !ok {
		return
	}

	slug, err := apiSlugFromPath(r.URL.Path, "/api/v1/skills/")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "skill slug 不正确")
		return
	}
	tenantID, err := resolveAPITenantScopeForUser(authCtx.User, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	skill, err := db.GetSkillBySlugForAPI(slug, tenantID)
	if err != nil {
		slog.Error("api skill detail failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "读取 skill 失败")
		return
	}
	if skill == nil {
		writeAPIError(w, http.StatusNotFound, "skill 不存在")
		return
	}

	readme, err := security.RenderSkillMarkdown(skillMarkdownSource(*skill))
	if err != nil {
		slog.Error("api skill render failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "渲染 skill 失败")
		return
	}

	writeJSON(w, http.StatusOK, apiSkillDetail{
		ID:            skill.ID,
		Slug:          skill.Slug,
		Name:          skill.Name,
		Description:   skill.Description,
		Version:       skill.Version,
		Author:        skill.Author,
		Readme:        string(readme),
		Rating:        skill.RatingAvg,
		RatingCount:   skill.RatingCount,
		DownloadCount: skill.DownloadCount,
		Category:      skill.Categories,
		Keywords:      buildSkillKeywords(*skill),
	})
}

func APIV1DownloadHandler(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := requireAPIKeyAuth(w, r)
	if !ok {
		return
	}

	idText, err := apiSlugFromPath(r.URL.Path, "/api/v1/download/")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "skill id 不正确")
		return
	}
	skillID, err := parseInt64(idText)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "skill id 不正确")
		return
	}

	skill, err := db.GetSkillByIDForAPI(skillID, authCtx.User.ID)
	if err != nil {
		slog.Error("api download failed", "error", err)
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
			slog.Error("read uploaded zip failed", "error", err)
			writeAPIError(w, http.StatusNotFound, "原始 zip 不存在")
			return
		}
		if err := db.IncrementSkillDownloadCount(skill.ID); err != nil {
			slog.Error("increment download count failed", "error", err)
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
		_, _ = w.Write(data)
		return
	}

	buf, err := buildSkillZIP(*skill)
	if err != nil {
		slog.Error("build sync zip failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "打包 zip 失败")
		return
	}
	if err := db.IncrementSkillDownloadCount(skill.ID); err != nil {
		slog.Error("increment download count failed", "error", err)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	_, _ = w.Write(buf.Bytes())
}

func APIV1UploadHandler(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := requireAPIKeyAuth(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	if !enforceRateLimit(w, fmt.Sprintf("user-write:%d", authCtx.User.ID), 10, authCtx.User.IsPlatformAdmin || authCtx.User.IsSubAdmin) {
		return
	}

	tenant, err := resolveRequiredAPITenantForUser(authCtx.User, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}
	if tenant == nil {
		writeAPIError(w, http.StatusForbidden, "当前用户没有可用租户")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeAPIError(w, http.StatusBadRequest, "文件大小超过 10MB 限制")
		return
	}
	file, header, err := r.FormFile("zipfile")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "请选择要上传的 ZIP 文件")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		writeAPIError(w, http.StatusBadRequest, "只支持 ZIP 格式的文件")
		return
	}
	buf, err := io.ReadAll(file)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "读取文件失败")
		return
	}
	skill, err := persistUploadedSkillArchive(tenant.TenantID, authCtx.User.ID, buf)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":            skill.ID,
		"slug":          skill.Slug,
		"review_status": "pending",
		"message":       "Skill 已提交审核，管理员通过后会在前台展示",
	})
}

func APIV1CategoriesHandler(w http.ResponseWriter, r *http.Request) {
	authCtx, ok := requireAPIKeyAuth(w, r)
	if !ok {
		return
	}
	tenantID, err := resolveAPITenantScopeForUser(authCtx.User, r)
	if err != nil {
		writeAPITenantScopeError(w, err)
		return
	}

	counts, err := db.ListCategoryCountsForAPI(tenantID)
	if err != nil {
		slog.Error("api categories failed", "error", err)
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

	writeJSON(w, http.StatusOK, map[string]interface{}{"categories": items})
}

func APIV1StatsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAPIKeyAuth(w, r); !ok {
		return
	}
	stats, err := db.GetPlatformStatsForAPI()
	if err != nil {
		slog.Error("api stats failed", "error", err)
		writeAPIError(w, http.StatusInternalServerError, "读取统计失败")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

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

func parsePositiveInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := parseInt64(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return int(parsed)
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
