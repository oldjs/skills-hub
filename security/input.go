package security

import (
	"html"
	"strings"
)

// 纯文本统一收一遍，存库时别把原始 HTML 带进去。
func EscapePlainText(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

// Markdown 要保留语法，但里面夹带的原始 HTML 还是先转义掉。
func EscapeMarkdownSource(value string) string {
	return html.EscapeString(normalizeMultiline(value))
}

// 老数据可能还没清洗过，这里统一揉成一份干净的 Markdown 源。
func CanonicalMarkdownSource(value string) string {
	return html.EscapeString(html.UnescapeString(normalizeMultiline(value)))
}

// 读出来给模板用时还原一下，避免实体再被模板转义一遍。
func DecodeStoredText(value string) string {
	return html.UnescapeString(value)
}

func normalizeMultiline(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
}
