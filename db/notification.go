package db

import (
	"time"

	"skills-hub/security"
)

// Notification 通知记录
type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Link      string    `json:"link"`
	IsRead    bool      `json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
}

// 创建通知
func CreateNotification(userID int64, nType, title, content, link string) error {
	title = security.EscapePlainText(title)
	content = security.EscapePlainText(content)
	_, err := GetDB().Exec(`
		INSERT INTO notifications (user_id, type, title, content, link)
		VALUES (?, ?, ?, ?, ?)
	`, userID, nType, title, content, link)
	return err
}

// 拉用户通知列表
func GetUserNotifications(userID int64, limit int) ([]Notification, error) {
	rows, err := GetDB().Query(`
		SELECT id, user_id, type, title, content, link, is_read, created_at
		FROM notifications WHERE user_id = ?
		ORDER BY created_at DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Notification
	for rows.Next() {
		var n Notification
		var isRead int
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.Link, &isRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.IsRead = isRead == 1
		n.Title = security.DecodeStoredText(n.Title)
		n.Content = security.DecodeStoredText(n.Content)
		list = append(list, n)
	}
	return list, rows.Err()
}

// 未读通知计数
func CountUnreadNotifications(userID int64) int {
	var count int
	_ = GetDB().QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0`, userID).Scan(&count)
	return count
}

// 标记单条通知已读
func MarkNotificationRead(notificationID, userID int64) error {
	_, err := GetDB().Exec(`UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?`, notificationID, userID)
	return err
}

// 全部标记已读
func MarkAllNotificationsRead(userID int64) error {
	_, err := GetDB().Exec(`UPDATE notifications SET is_read = 1 WHERE user_id = ? AND is_read = 0`, userID)
	return err
}
