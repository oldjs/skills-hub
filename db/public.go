package db

// 返回第一个 active 租户的 ID，给匿名用户兜底
func GetDefaultTenantID() int64 {
	var id int64
	err := GetDB().QueryRow(`SELECT id FROM tenants WHERE status = 'active' ORDER BY id ASC LIMIT 1`).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}
