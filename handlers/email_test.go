package handlers

import "testing"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"123@qq.com", true},
		{"test@gmail.com", true},
		{"", false},
		{"noat", false},
		{"@no-local.com", false},
	}
	for _, tt := range tests {
		if got := validateEmail(tt.email); got != tt.want {
			t.Errorf("validateEmail(%q) = %v, want %v", tt.email, got, tt.want)
		}
	}
}

func TestValidateEmailForRegistration(t *testing.T) {
	tests := []struct {
		email string
		ok    bool
		desc  string
	}{
		{"123456@qq.com", true, "纯数字QQ"},
		{"abc@qq.com", false, "非数字QQ被拒"},
		{"user@gmail.com", true, "正常Gmail"},
		{"user+tag@gmail.com", false, "Gmail别名被拒"},
		{"u.ser@gmail.com", true, "Gmail带点允许"},
		{"user@outlook.com", false, "Outlook被拒"},
		{"user@yahoo.com", false, "Yahoo被拒"},
	}
	for _, tt := range tests {
		ok, msg := validateEmailForRegistration(tt.email)
		if ok != tt.ok {
			t.Errorf("%s: validateEmailForRegistration(%q) = %v (%s), want %v", tt.desc, tt.email, ok, msg, tt.ok)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User@Example.COM", "user@example.com"},
		{"  test@gmail.com  ", "test@gmail.com"},
		// Gmail 规范化：去掉点号
		{"u.s.e.r@gmail.com", "user@gmail.com"},
		// Gmail 规范化：去掉 + 别名
		{"user+tag@gmail.com", "user@gmail.com"},
		// QQ 邮箱不做规范化（除了 lowercase）
		{"123456@QQ.COM", "123456@qq.com"},
	}
	for _, tt := range tests {
		got := normalizeEmail(tt.input)
		if got != tt.want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeGmailAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@gmail.com", "user@gmail.com"},
		{"u.ser@gmail.com", "user@gmail.com"},
		{"u.s.e.r@gmail.com", "user@gmail.com"},
		{"user+tag@gmail.com", "user@gmail.com"},
		{"u.ser+tag@gmail.com", "user@gmail.com"},
	}
	for _, tt := range tests {
		got := normalizeGmailAddress(tt.input)
		if got != tt.want {
			t.Errorf("normalizeGmailAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
