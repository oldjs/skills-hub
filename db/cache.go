package db

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"skills-hub/models"

	"github.com/redis/go-redis/v9"
)

var cacheClient *redis.Client

// 设置缓存用的 Redis 客户端（和 session 共用同一个）
func SetCacheClient(client *redis.Client) {
	cacheClient = client
}

const queryCacheTTL = 60 * time.Second

// 缓存 key 前缀
func skillsCacheKey(tenantID int64, query, category, sort string, page, perPage int) string {
	return "cache:skills:" + json.Number(json.Number(string(rune(tenantID)))).String() +
		":" + query + ":" + category + ":" + sort
}

// 尝试从缓存读搜索结果
func GetCachedSkills(tenantID int64, cacheTag string) ([]models.Skill, int, bool) {
	if cacheClient == nil {
		return nil, 0, false
	}
	key := "cache:skills:" + cacheTag
	data, err := cacheClient.Get(context.Background(), key).Bytes()
	if err != nil {
		return nil, 0, false
	}

	var cached struct {
		Skills []models.Skill `json:"skills"`
		Total  int            `json:"total"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, 0, false
	}
	return cached.Skills, cached.Total, true
}

// 写入缓存
func SetCachedSkills(cacheTag string, skills []models.Skill, total int) {
	if cacheClient == nil {
		return
	}
	cached := struct {
		Skills []models.Skill `json:"skills"`
		Total  int            `json:"total"`
	}{Skills: skills, Total: total}

	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	key := "cache:skills:" + cacheTag
	if err := cacheClient.Set(context.Background(), key, data, queryCacheTTL).Err(); err != nil {
		slog.Error("cache write failed", "key", key, "error", err)
	}
}
