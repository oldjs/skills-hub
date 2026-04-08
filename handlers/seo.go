package handlers

import (
	"os"
	"strings"
)

// 站点根 URL，用于生成 canonical / og:url
func siteBaseURL() string {
	base := os.Getenv("SITE_URL")
	if base == "" {
		base = "https://skills-hub.example.com"
	}
	return strings.TrimRight(base, "/")
}

// 拼完整 canonical URL
func canonicalURL(path string) string {
	return siteBaseURL() + path
}

// 截断文本到指定长度，给 meta description 用
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
