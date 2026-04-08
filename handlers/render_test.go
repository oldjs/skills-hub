package handlers

import "testing"

func TestInitTemplatesParsesAllTemplates(t *testing.T) {
	InitTemplates("../templates")
	if len(templates) == 0 {
		t.Fatal("expected templates to be initialized")
	}
}

func TestCategoryList(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"browser", 1},
		{"browser, agent, api", 3},
		{" ,  , ", 0},
	}
	for _, tt := range tests {
		got := categoryList(tt.input)
		if len(got) != tt.want {
			t.Errorf("categoryList(%q) len = %d, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "u***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"", "***"},
		{"nope", "***"},
	}
	for _, tt := range tests {
		got := maskEmail(tt.input)
		if got != tt.want {
			t.Errorf("maskEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFirstChar(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello", "H"},
		{"  hello  ", "h"},
		{"", "?"},
		{"中文", "中"},
	}
	for _, tt := range tests {
		got := firstChar(tt.input)
		if got != tt.want {
			t.Errorf("firstChar(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRoundRating(t *testing.T) {
	tests := []struct {
		input float64
		want  int
	}{
		{0.0, 0},
		{3.4, 3},
		{3.5, 4},
		{4.9, 5},
	}
	for _, tt := range tests {
		got := roundRating(tt.input)
		if got != tt.want {
			t.Errorf("roundRating(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
