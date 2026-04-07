package security

import (
	"strings"
	"testing"
)

func TestRenderCommentMarkdownRemovesDangerousContent(t *testing.T) {
	rendered, err := RenderCommentMarkdown(`[点我](javascript:alert(1)) <img src=x onerror=alert(1)>`)
	if err != nil {
		t.Fatalf("RenderCommentMarkdown returned error: %v", err)
	}

	html := string(rendered)
	if strings.Contains(html, `href="javascript:`) {
		t.Fatalf("javascript link should be removed, got: %s", html)
	}
	if strings.Contains(strings.ToLower(html), "<img") {
		t.Fatalf("dangerous html tag should not be rendered, got: %s", html)
	}
}

func TestRenderSkillMarkdownStripsFrontmatter(t *testing.T) {
	rendered, err := RenderSkillMarkdown("---\nname: demo\n---\n# Demo\n\n正文")
	if err != nil {
		t.Fatalf("RenderSkillMarkdown returned error: %v", err)
	}

	html := string(rendered)
	if strings.Contains(html, "name: demo") {
		t.Fatalf("frontmatter should not appear in rendered HTML, got: %s", html)
	}
	if !strings.Contains(html, "Demo") {
		t.Fatalf("rendered HTML should contain markdown body, got: %s", html)
	}
}
