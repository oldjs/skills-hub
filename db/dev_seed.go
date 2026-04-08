package db

import (
	"database/sql"
	"log"
)

// EnsureDevSeed 在 DEV_MODE 启动时自动创建开发用的租户和用户
// 只在数据库为空时生效，不会覆盖已有数据
func EnsureDevSeed() error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先看有没有租户，有就不管了
	var tenantCount int
	if err := tx.QueryRow(`SELECT COUNT(1) FROM tenants`).Scan(&tenantCount); err != nil {
		return err
	}
	if tenantCount > 0 {
		log.Println("[DEV_MODE] tenants already exist, skip seeding")
		return nil
	}

	// 建一个默认租户
	tenantID, err := ensureDevTenantTx(tx)
	if err != nil {
		return err
	}

	// 建一个开发用户
	userID, err := ensureDevUserTx(tx)
	if err != nil {
		return err
	}

	// 把用户加到租户里
	if err := ensureDevMembershipTx(tx, tenantID, userID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("[DEV_MODE] seed done: tenant_id=%d, user_id=%d (dev@localhost / platform admin)", tenantID, userID)
	return nil
}

// 建一个开发租户
func ensureDevTenantTx(tx *sql.Tx) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM tenants WHERE slug = 'dev'`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := tx.Exec(`INSERT INTO tenants (slug, name, description, status) VALUES ('dev', 'Dev Tenant', 'Local development tenant', 'active')`)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// 建一个开发用户，默认 platform admin
func ensureDevUserTx(tx *sql.Tx) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM users WHERE email = 'dev@localhost'`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := tx.Exec(`INSERT INTO users (email, display_name, status, is_platform_admin) VALUES ('dev@localhost', 'Dev', 'active', 1)`)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// 把用户加到租户，角色 owner
func ensureDevMembershipTx(tx *sql.Tx, tenantID, userID int64) error {
	var exists int
	err := tx.QueryRow(`SELECT COUNT(1) FROM tenant_members WHERE tenant_id = ? AND user_id = ?`, tenantID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}

	_, err = tx.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (?, ?, 'owner', 'active')`, tenantID, userID)
	return err
}
