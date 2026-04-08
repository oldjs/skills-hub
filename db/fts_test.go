package db

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  ", ""},
		{"browser", `"browser"*`},
		{"browser agent", `"browser"* AND "agent"*`},
		{"web scraping tool", `"web"* AND "scraping"* AND "tool"*`},
	}
	for _, tt := range tests {
		got := buildFTSQuery(tt.input)
		if got != tt.want {
			t.Errorf("buildFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitCategories(t *testing.T) {
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
		got := splitCategories(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitCategories(%q) len = %d, want %d", tt.input, len(got), tt.want)
		}
	}
}
