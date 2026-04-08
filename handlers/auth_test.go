package handlers

import (
	"testing"
	"time"
)

func TestSecondsRemaining(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want int
	}{
		{0, 1},
		{-1 * time.Second, 1},
		{500 * time.Millisecond, 1},
		{1 * time.Second, 1},
		{1500 * time.Millisecond, 2},
		{59 * time.Second, 59},
		{60 * time.Second, 60},
	}
	for _, tt := range tests {
		got := secondsRemaining(tt.dur)
		if got != tt.want {
			t.Errorf("secondsRemaining(%v) = %d, want %d", tt.dur, got, tt.want)
		}
	}
}

func TestSafeLocalRedirect(t *testing.T) {
	tests := []struct {
		referrer string
		host     string
		want     string
	}{
		{"", "localhost", "/"},
		{"/search?q=test", "localhost", "/search?q=test"},
		{"/admin", "localhost", "/admin"},
		{"https://evil.com/steal", "localhost", "/"},
		{"https://localhost/safe", "localhost", "/safe"},
		{"relative", "localhost", "/"},
	}
	for _, tt := range tests {
		got := safeLocalRedirect(tt.referrer, tt.host)
		if got != tt.want {
			t.Errorf("safeLocalRedirect(%q, %q) = %q, want %q", tt.referrer, tt.host, got, tt.want)
		}
	}
}

func TestRateLimitKey(t *testing.T) {
	// 同样的输入应该产生同样的 key
	k1 := rateLimitKey("prefix:", "a", "b")
	k2 := rateLimitKey("prefix:", "a", "b")
	if k1 != k2 {
		t.Error("same inputs should produce same key")
	}
	// 不同输入产生不同 key
	k3 := rateLimitKey("prefix:", "a", "c")
	if k1 == k3 {
		t.Error("different inputs should produce different keys")
	}
}
