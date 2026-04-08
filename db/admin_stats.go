package db

import "time"

// 7 日或 30 日的日期+计数数据点
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// 活跃用户排行
type ActiveUser struct {
	UserID      int64  `json:"userId"`
	DisplayName string `json:"displayName"`
	ActionCount int    `json:"actionCount"`
}

// 最近 N 天每天新注册用户数
func GetDailyRegistrations(days int) ([]DailyCount, error) {
	rows, err := GetDB().Query(`
		SELECT DATE(created_at) as d, COUNT(*) as c
		FROM users
		WHERE created_at >= datetime('now', ? || ' days')
		GROUP BY d ORDER BY d ASC
	`, -days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := fillDateGaps(days, rows)
	return result, nil
}

// 最近 N 天每天新增技能数
func GetDailySkillGrowth(days int) ([]DailyCount, error) {
	rows, err := GetDB().Query(`
		SELECT DATE(created_at) as d, COUNT(*) as c
		FROM skills
		WHERE created_at >= datetime('now', ? || ' days')
		GROUP BY d ORDER BY d ASC
	`, -days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := fillDateGaps(days, rows)
	return result, nil
}

// 最近 7 天最活跃的用户（评分+评论数排行）
func GetMostActiveUsers(days, limit int) ([]ActiveUser, error) {
	rows, err := GetDB().Query(`
		SELECT u.id, u.display_name, (
			(SELECT COUNT(*) FROM skill_ratings r WHERE r.user_id = u.id AND r.created_at >= datetime('now', ? || ' days')) +
			(SELECT COUNT(*) FROM skill_comments c WHERE c.user_id = u.id AND c.created_at >= datetime('now', ? || ' days'))
		) as action_count
		FROM users u
		HAVING action_count > 0
		ORDER BY action_count DESC
		LIMIT ?
	`, -days, -days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ActiveUser
	for rows.Next() {
		var u ActiveUser
		if err := rows.Scan(&u.UserID, &u.DisplayName, &u.ActionCount); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// 补齐没有数据的日期（填 0）
func fillDateGaps(days int, rows interface{ Next() bool; Scan(...interface{}) error }) []DailyCount {
	// 先收集有数据的日期
	data := make(map[string]int)
	for rows.Next() {
		var d string
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			continue
		}
		data[d] = c
	}

	// 生成完整日期序列
	result := make([]DailyCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		count := data[date]
		result = append(result, DailyCount{Date: date, Count: count})
	}
	return result
}
