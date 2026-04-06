package security

import (
	"bytes"
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

var markdownSanitizer = func() *bluemonday.Policy {
	// 渲染完再走一遍白名单，script 和事件属性这类脏东西直接丢掉。
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class", "style").Globally()
	return policy
}()

var markdownRenderer = goldmark.New(
	// 常见 Markdown 能力和代码高亮一起开，详情页和评论共用一套输出。
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(false),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	// 不开 unsafe，goldmark 会把原始 HTML 和危险链接拦掉。
	goldmark.WithRendererOptions(
		gmhtml.WithHardWraps(),
	),
)

// Skill 文档会带 frontmatter，渲染前先把这段元数据剥掉。
func RenderSkillMarkdown(source string) (template.HTML, error) {
	body := stripFrontmatter(CanonicalMarkdownSource(source))
	return renderMarkdown(body)
}

// 评论直接走同一套安全渲染，预览和最终展示保持一致。
func RenderCommentMarkdown(source string) (template.HTML, error) {
	return renderMarkdown(CanonicalMarkdownSource(source))
}

func renderMarkdown(source string) (template.HTML, error) {
	if strings.TrimSpace(source) == "" {
		return template.HTML(""), nil
	}

	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &buf); err != nil {
		return "", err
	}

	// 高亮标签留下，其他危险 HTML 再清一遍。
	return template.HTML(markdownSanitizer.Sanitize(buf.String())), nil
}

func stripFrontmatter(source string) string {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "---") {
		return source
	}

	parts := strings.SplitN(trimmed, "---", 3)
	if len(parts) < 3 {
		return source
	}
	return strings.TrimSpace(parts[2])
}
