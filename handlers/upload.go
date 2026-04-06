package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"skills-hub/db"
)

const maxUploadSize = 10 << 20 // 10MB

// GET 显示上传页面，POST 处理上传
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	sess := GetCurrentSession(r)
	if sess == nil || sess.CurrentTenantID == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		data := PageData{
			Title:       "上传 Skill - Skills Hub",
			CurrentPage: "upload",
		}
		RenderTemplate(w, r, "upload.html", data)
		return
	}

	if !ValidateCSRFToken(r) {
		http.Error(w, "无效的请求", http.StatusForbidden)
		return
	}

	// 限制请求体大小
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		renderUploadError(w, r, "文件大小超过 10MB 限制")
		return
	}

	// 校验图形验证码
	captchaInput := strings.TrimSpace(r.FormValue("captcha"))
	if !validateCaptcha(r, captchaInput) {
		renderUploadError(w, r, "图形验证码错误，请重新输入")
		return
	}

	file, header, err := r.FormFile("zipfile")
	if err != nil {
		renderUploadError(w, r, "请选择要上传的 ZIP 文件")
		return
	}
	defer file.Close()

	// 校验文件扩展名
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		renderUploadError(w, r, "只支持 ZIP 格式的文件")
		return
	}

	// 读到内存里
	buf, err := io.ReadAll(file)
	if err != nil {
		renderUploadError(w, r, "读取文件失败")
		return
	}

	// 解析 ZIP
	zipReader, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		renderUploadError(w, r, "无效的 ZIP 文件")
		return
	}

	// 找 SKILL.md
	skillMD, err := findSkillMD(zipReader)
	if err != nil {
		renderUploadError(w, r, err.Error())
		return
	}

	// 解析 SKILL.md 提取元数据
	meta := parseSkillMD(skillMD)
	if meta.Name == "" {
		renderUploadError(w, r, "SKILL.md 中未找到技能名称，请在第一行使用 # 标题")
		return
	}

	// 生成 slug
	slug := generateSlug(meta.Name)

	// 保存 ZIP 到磁盘
	uploadDir := fmt.Sprintf("./uploads/%d", sess.CurrentTenantID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("创建上传目录失败: %v", err)
		renderUploadError(w, r, "保存文件失败")
		return
	}
	zipPath := filepath.Join(uploadDir, slug+".zip")
	if err := os.WriteFile(zipPath, buf, 0644); err != nil {
		log.Printf("保存 ZIP 失败: %v", err)
		renderUploadError(w, r, "保存文件失败")
		return
	}

	// 分类：如果 SKILL.md 里没提取到，用自动分类
	categories := meta.Categories
	if categories == "" {
		categories = db.CategorizeByText(meta.Name, meta.Description)
	}

	// 存数据库
	skill, err := db.SaveUploadedSkill(sess.CurrentTenantID, slug, meta.Name, meta.Description, skillMD, meta.Version, categories)
	if err != nil {
		renderUploadError(w, r, err.Error())
		return
	}

	// 上传成功，跳到详情页
	http.Redirect(w, r, "/skill?slug="+skill.Slug, http.StatusSeeOther)
}

// 渲染上传页面带错误信息
func renderUploadError(w http.ResponseWriter, r *http.Request, errMsg string) {
	data := PageData{
		Title:       "上传 Skill - Skills Hub",
		CurrentPage: "upload",
		Error:       errMsg,
	}
	RenderTemplate(w, r, "upload.html", data)
}

// 在 ZIP 里找 SKILL.md，支持根目录或一级子目录
func findSkillMD(zr *zip.Reader) (string, error) {
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		// 只看根目录或一级子目录下的 SKILL.md
		depth := strings.Count(strings.TrimSuffix(f.Name, "/"), "/")
		if strings.EqualFold(name, "SKILL.md") && depth <= 1 && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("读取 SKILL.md 失败")
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("读取 SKILL.md 失败")
			}
			return string(content), nil
		}
	}
	return "", fmt.Errorf("ZIP 中未找到 SKILL.md 文件")
}

// SKILL.md 的元数据
type skillMeta struct {
	Name        string
	Description string
	Version     string
	Categories  string
}

// 解析 SKILL.md，支持 frontmatter 和纯 markdown
func parseSkillMD(content string) skillMeta {
	meta := skillMeta{}

	// 尝试解析 frontmatter（--- 开头的 YAML 块）
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		parts := strings.SplitN(strings.TrimSpace(content), "---", 3)
		if len(parts) >= 3 {
			// parts[1] 是 frontmatter 内容
			frontmatter := parts[1]
			meta.Name = extractFrontmatterField(frontmatter, "name")
			meta.Description = extractFrontmatterField(frontmatter, "description")
			meta.Version = extractFrontmatterField(frontmatter, "version")
			meta.Categories = extractFrontmatterField(frontmatter, "categories")

			// 如果 frontmatter 没给 description，从正文取
			if meta.Description == "" {
				body := strings.TrimSpace(parts[2])
				meta.Description = extractFirstParagraph(body)
			}
		}
	}

	// 如果 frontmatter 没有 name，从 markdown 标题取
	if meta.Name == "" {
		meta.Name = extractMarkdownTitle(content)
	}

	// 如果还没有 description，取第一段正文
	if meta.Description == "" {
		meta.Description = extractFirstParagraph(content)
	}

	// description 截断到合理长度
	if len([]rune(meta.Description)) > 200 {
		meta.Description = string([]rune(meta.Description)[:200]) + "..."
	}

	return meta
}

// 从 frontmatter 提取字段值
func extractFrontmatterField(fm, field string) string {
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		prefix := field + ":"
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			val := strings.TrimSpace(line[len(prefix):])
			// 去掉引号
			val = strings.Trim(val, "\"'")
			return val
		}
	}
	return ""
}

// 从 markdown 提取第一个 # 标题
func extractMarkdownTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}

// 提取第一段非标题、非空行的文本
func extractFirstParagraph(content string) string {
	var paragraph []string
	inParagraph := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// 跳过标题行和空行
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			if inParagraph && len(paragraph) > 0 {
				break
			}
			continue
		}
		// 跳过 frontmatter 分隔符
		if trimmed == "---" {
			continue
		}
		inParagraph = true
		paragraph = append(paragraph, trimmed)
	}

	return strings.Join(paragraph, " ")
}

// 把名称转成 slug
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	// 只保留字母、数字、连字符
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	// 压缩连续连字符
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if slug == "" {
		slug = "skill"
	}
	return slug
}
