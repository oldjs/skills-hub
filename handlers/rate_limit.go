package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const rateLimitWindow = time.Minute

func allowRateLimit(key string, limit int) (bool, int, error) {
	if limit <= 0 {
		return true, 0, nil
	}
	now := time.Now().UTC()
	windowKey := fmt.Sprintf("ratelimit:%s:%s", key, now.Format("200601021504"))
	retryAfter := int(rateLimitWindow.Seconds()) - now.Second()
	if retryAfter <= 0 {
		retryAfter = 1
	}

	ctx := context.Background()
	count, err := rdb.Incr(ctx, windowKey).Result()
	if err != nil {
		return false, retryAfter, err
	}
	if count == 1 {
		if err := rdb.Expire(ctx, windowKey, rateLimitWindow).Err(); err != nil {
			return false, retryAfter, err
		}
	}
	if count > int64(limit) {
		return false, retryAfter, nil
	}
	return true, retryAfter, nil
}

func enforceRateLimit(w http.ResponseWriter, key string, limit int, exempt bool) bool {
	if exempt {
		return true
	}
	allowed, retryAfter, err := allowRateLimit(key, limit)
	if err != nil {
		return true
	}
	if allowed {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	http.Error(w, "请求过于频繁，请稍后再试", http.StatusTooManyRequests)
	return false
}
