package handlers

import "testing"

func TestInitTemplatesParsesAllTemplates(t *testing.T) {
	InitTemplates("../templates")
	if len(templates) == 0 {
		t.Fatal("expected templates to be initialized")
	}
}
