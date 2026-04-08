package handlers

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"time"

	"skills-hub/db"
)

// sitemap.xml 的 XML 结构
type urlSet struct {
	XMLName xml.Name  `xml:"urlset"`
	XMLNS   string    `xml:"xmlns,attr"`
	URLs    []siteURL `xml:"url"`
}

type siteURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// GET /sitemap.xml
func SitemapHandler(w http.ResponseWriter, r *http.Request) {
	base := siteBaseURL()

	urls := []siteURL{
		// 静态页面
		{Loc: base + "/", ChangeFreq: "daily", Priority: "1.0"},
		{Loc: base + "/search", ChangeFreq: "daily", Priority: "0.8"},
		{Loc: base + "/login", ChangeFreq: "monthly", Priority: "0.3"},
		{Loc: base + "/register", ChangeFreq: "monthly", Priority: "0.3"},
	}

	// 动态页面：所有已审核通过的技能详情页
	skills, err := db.GetAllApprovedSkillSlugs()
	if err != nil {
		slog.Error("sitemap skill load failed", "error", err)
	} else {
		for _, s := range skills {
			urls = append(urls, siteURL{
				Loc:        base + "/skill?slug=" + s.Slug,
				LastMod:    s.UpdatedAt.Format(time.DateOnly),
				ChangeFreq: "weekly",
				Priority:   "0.7",
			})
		}
	}

	sitemap := urlSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(sitemap)
}

// GET /robots.txt
func RobotsTxtHandler(w http.ResponseWriter, r *http.Request) {
	base := siteBaseURL()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write([]byte("User-agent: *\n"))
	w.Write([]byte("Allow: /\n"))
	w.Write([]byte("Allow: /search\n"))
	w.Write([]byte("Allow: /skill\n"))
	w.Write([]byte("\n"))
	// 禁止爬后台和 API
	w.Write([]byte("Disallow: /admin\n"))
	w.Write([]byte("Disallow: /api/\n"))
	w.Write([]byte("Disallow: /account\n"))
	w.Write([]byte("Disallow: /upload\n"))
	w.Write([]byte("Disallow: /switch-tenant\n"))
	w.Write([]byte("Disallow: /send-code\n"))
	w.Write([]byte("Disallow: /captcha\n"))
	w.Write([]byte("\n"))
	w.Write([]byte("Sitemap: " + base + "/sitemap.xml\n"))
}
