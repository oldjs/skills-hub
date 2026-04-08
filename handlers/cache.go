package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"
)

// 给技能详情页设置 ETag + Last-Modified
func setCacheHeaders(w http.ResponseWriter, r *http.Request, content string, lastModified time.Time) bool {
	// ETag 基于内容 hash
	hash := sha256.Sum256([]byte(content))
	etag := fmt.Sprintf(`"%x"`, hash[:8])

	w.Header().Set("ETag", etag)
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}
	// 私有缓存，10 分钟内可用，之后需要 revalidate
	w.Header().Set("Cache-Control", "private, max-age=600, must-revalidate")

	// 检查 If-None-Match
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	// 检查 If-Modified-Since
	if ims := r.Header.Get("If-Modified-Since"); ims != "" && !lastModified.IsZero() {
		t, err := http.ParseTime(ims)
		if err == nil && !lastModified.After(t) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	return false
}

// 搜索结果用短 TTL 公共缓存
func setSearchCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=30")
}
