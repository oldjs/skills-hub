package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"skills-hub/models"
	"skills-hub/security"
)

func CreateTenant(slug, name, description string, autoSyncEnabled bool) (*models.Tenant, error) {
	// 租户信息在后台和头部切换器都会展示，入库前先转义。
	slug = security.EscapePlainText(slug)
	name = security.EscapePlainText(name)
	description = security.EscapePlainText(description)

	result, err := GetDB().Exec(`
		INSERT INTO tenants (slug, name, description, auto_sync_enabled)
		VALUES (?, ?, ?, ?)
	`, slug, name, description, boolToInt(autoSyncEnabled))
	if err != nil {
		return nil, err
	}

	tenantID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetTenantByID(tenantID)
}

func CreatePersonalTenantForUser(userID int64, displayName, email string) (*models.Tenant, error) {
	baseName := strings.TrimSpace(displayName)
	if baseName == "" {
		baseName = strings.Split(email, "@")[0]
	}
	slug := uniqueTenantSlug(baseName)
	name := fmt.Sprintf("%s Workspace", strings.TrimSpace(baseName))
	tenant, err := CreateTenant(slug, name, "系统自动创建的个人租户", true)
	if err != nil {
		return nil, err
	}
	if _, err := AddTenantMember(tenant.ID, userID, "owner"); err != nil {
		return nil, err
	}
	return tenant, nil
}

func uniqueTenantSlug(seed string) string {
	seed = strings.ToLower(strings.TrimSpace(seed))
	seed = strings.ReplaceAll(seed, "@", "-")
	seed = strings.ReplaceAll(seed, ".", "-")
	seed = strings.ReplaceAll(seed, "_", "-")
	seed = strings.ReplaceAll(seed, " ", "-")
	seed = strings.Trim(seed, "-")
	if seed == "" {
		seed = "workspace"
	}
	for strings.Contains(seed, "--") {
		seed = strings.ReplaceAll(seed, "--", "-")
	}

	slug := seed
	index := 1
	for {
		exists, _ := tenantSlugExists(slug)
		if !exists {
			return slug
		}
		index++
		slug = fmt.Sprintf("%s-%d", seed, index)
	}
}

func tenantSlugExists(slug string) (bool, error) {
	row := GetDB().QueryRow(`SELECT COUNT(1) FROM tenants WHERE slug = ?`, slug)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func GetTenantByID(tenantID int64) (*models.Tenant, error) {
	row := GetDB().QueryRow(`
		SELECT id, slug, name, description, status, auto_sync_enabled, created_at, updated_at
		FROM tenants WHERE id = ?
	`, tenantID)

	var tenant models.Tenant
	var autoSyncEnabled int
	if err := row.Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Description, &tenant.Status, &autoSyncEnabled, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	tenant.AutoSyncEnabled = autoSyncEnabled == 1
	decodeTenantForDisplay(&tenant)
	return &tenant, nil
}

func ListTenants() ([]models.Tenant, error) {
	rows, err := GetDB().Query(`
		SELECT id, slug, name, description, status, auto_sync_enabled, created_at, updated_at
		FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []models.Tenant
	for rows.Next() {
		var tenant models.Tenant
		var autoSyncEnabled int
		if err := rows.Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Description, &tenant.Status, &autoSyncEnabled, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, err
		}
		tenant.AutoSyncEnabled = autoSyncEnabled == 1
		decodeTenantForDisplay(&tenant)
		tenants = append(tenants, tenant)
	}

	return tenants, rows.Err()
}

func ListUserTenants(userID int64) ([]models.UserTenant, error) {
	rows, err := GetDB().Query(`
		SELECT tm.tenant_id, t.slug, t.name, t.status, tm.role, tm.status
		FROM tenant_members tm
		JOIN tenants t ON t.id = tm.tenant_id
		WHERE tm.user_id = ?
		ORDER BY t.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []models.UserTenant
	for rows.Next() {
		var item models.UserTenant
		if err := rows.Scan(&item.TenantID, &item.TenantSlug, &item.TenantName, &item.TenantStatus, &item.TenantRole, &item.MembershipStatus); err != nil {
			return nil, err
		}
		decodeUserTenantForDisplay(&item)
		tenants = append(tenants, item)
	}

	return tenants, rows.Err()
}

func GetUserTenant(userID, tenantID int64) (*models.UserTenant, error) {
	row := GetDB().QueryRow(`
		SELECT tm.tenant_id, t.slug, t.name, t.status, tm.role, tm.status
		FROM tenant_members tm
		JOIN tenants t ON t.id = tm.tenant_id
		WHERE tm.user_id = ? AND tm.tenant_id = ?
	`, userID, tenantID)

	var item models.UserTenant
	if err := row.Scan(&item.TenantID, &item.TenantSlug, &item.TenantName, &item.TenantStatus, &item.TenantRole, &item.MembershipStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	decodeUserTenantForDisplay(&item)
	return &item, nil
}

func PickActiveTenant(userID int64, lastTenantID *int64) (*models.UserTenant, error) {
	tenants, err := ListUserTenants(userID)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		if tenant.TenantStatus != "active" || tenant.MembershipStatus != "active" {
			continue
		}
		if lastTenantID != nil && tenant.TenantID == *lastTenantID {
			return &tenant, nil
		}
	}

	for _, tenant := range tenants {
		if tenant.TenantStatus == "active" && tenant.MembershipStatus == "active" {
			copy := tenant
			return &copy, nil
		}
	}

	return nil, nil
}

func AddTenantMember(tenantID, userID int64, role string) (*models.TenantMember, error) {
	role = security.EscapePlainText(role)

	_, err := GetDB().Exec(`
		INSERT INTO tenant_members (tenant_id, user_id, role, status)
		VALUES (?, ?, ?, 'active')
		ON CONFLICT(tenant_id, user_id) DO UPDATE SET role = excluded.role, status = 'active', updated_at = CURRENT_TIMESTAMP
	`, tenantID, userID, role)
	if err != nil {
		return nil, err
	}

	return GetTenantMember(tenantID, userID)
}

func GetTenantMember(tenantID, userID int64) (*models.TenantMember, error) {
	row := GetDB().QueryRow(`
		SELECT tm.id, tm.tenant_id, tm.user_id, tm.role, tm.status, tm.joined_at, tm.updated_at, u.email, u.display_name
		FROM tenant_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = ? AND tm.user_id = ?
	`, tenantID, userID)

	var member models.TenantMember
	if err := row.Scan(&member.ID, &member.TenantID, &member.UserID, &member.Role, &member.Status, &member.JoinedAt, &member.UpdatedAt, &member.Email, &member.DisplayName); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	decodeTenantMemberForDisplay(&member)
	return &member, nil
}

func ListTenantMembers(tenantID int64) ([]models.TenantMember, error) {
	rows, err := GetDB().Query(`
		SELECT tm.id, tm.tenant_id, tm.user_id, tm.role, tm.status, tm.joined_at, tm.updated_at, u.email, u.display_name
		FROM tenant_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = ?
		ORDER BY CASE tm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.TenantMember
	for rows.Next() {
		var member models.TenantMember
		if err := rows.Scan(&member.ID, &member.TenantID, &member.UserID, &member.Role, &member.Status, &member.JoinedAt, &member.UpdatedAt, &member.Email, &member.DisplayName); err != nil {
			return nil, err
		}
		decodeTenantMemberForDisplay(&member)
		members = append(members, member)
	}

	return members, rows.Err()
}

func UpdateTenantMember(tenantID, userID int64, role, status string) error {
	role = security.EscapePlainText(role)
	status = security.EscapePlainText(status)

	_, err := GetDB().Exec(`
		UPDATE tenant_members
		SET role = ?, status = ?, updated_at = ?
		WHERE tenant_id = ? AND user_id = ?
	`, role, status, time.Now(), tenantID, userID)
	return err
}

func RemoveTenantMember(tenantID, userID int64) error {
	row := GetDB().QueryRow(`SELECT COUNT(1) FROM tenant_members WHERE tenant_id = ? AND role = 'owner' AND status = 'active'`, tenantID)
	var owners int
	if err := row.Scan(&owners); err != nil {
		return err
	}

	member, err := GetTenantMember(tenantID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return nil
	}
	if member.Role == "owner" && owners <= 1 {
		return errors.New("至少保留一个 owner")
	}

	_, err = GetDB().Exec(`DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`, tenantID, userID)
	return err
}

func CreateTenantInvite(tenantID int64, email, role string, expiresAt time.Time) error {
	email = security.EscapePlainText(email)
	role = security.EscapePlainText(role)

	result, err := GetDB().Exec(`
		UPDATE tenant_invites
		SET role = ?, expires_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND email = ? AND status = 'pending'
	`, role, expiresAt, tenantID, email)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		return nil
	}
	_, err = GetDB().Exec(`
		INSERT INTO tenant_invites (tenant_id, email, role, status, expires_at)
		VALUES (?, ?, ?, 'pending', ?)
	`, tenantID, email, role, expiresAt)
	return err
}

func ListTenantInvites(tenantID int64) ([]models.TenantInvite, error) {
	rows, err := GetDB().Query(`
		SELECT id, tenant_id, email, role, status, expires_at, accepted_at, created_at, updated_at
		FROM tenant_invites WHERE tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []models.TenantInvite
	for rows.Next() {
		var invite models.TenantInvite
		var acceptedAt sql.NullTime
		if err := rows.Scan(&invite.ID, &invite.TenantID, &invite.Email, &invite.Role, &invite.Status, &invite.ExpiresAt, &acceptedAt, &invite.CreatedAt, &invite.UpdatedAt); err != nil {
			return nil, err
		}
		if acceptedAt.Valid {
			invite.AcceptedAt = &acceptedAt.Time
		}
		decodeTenantInviteForDisplay(&invite)
		invites = append(invites, invite)
	}

	return invites, rows.Err()
}

func RevokeTenantInvite(inviteID int64) error {
	_, err := GetDB().Exec(`
		UPDATE tenant_invites
		SET status = 'revoked', updated_at = ?
		WHERE id = ?
	`, time.Now(), inviteID)
	return err
}

func AcceptPendingInvites(userID int64, email string) error {
	email = security.EscapePlainText(email)

	rows, err := GetDB().Query(`
		SELECT id, tenant_id, role FROM tenant_invites
		WHERE email = ? AND status = 'pending' AND expires_at > ?
	`, email, time.Now())
	if err != nil {
		return err
	}
	defer rows.Close()

	type inviteRow struct {
		id       int64
		tenantID int64
		role     string
	}

	var invites []inviteRow
	for rows.Next() {
		var item inviteRow
		if err := rows.Scan(&item.id, &item.tenantID, &item.role); err != nil {
			return err
		}
		invites = append(invites, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, invite := range invites {
		if _, err := AddTenantMember(invite.tenantID, userID, invite.role); err != nil {
			return err
		}
		if _, err := GetDB().Exec(`
			UPDATE tenant_invites
			SET status = 'accepted', accepted_at = ?, updated_at = ?
			WHERE id = ?
		`, time.Now(), time.Now(), invite.id); err != nil {
			return err
		}
	}

	return nil
}

func UpdateTenant(tenantID int64, name, slug, description, status string, autoSyncEnabled bool) error {
	name = security.EscapePlainText(name)
	slug = security.EscapePlainText(slug)
	description = security.EscapePlainText(description)
	status = security.EscapePlainText(status)

	_, err := GetDB().Exec(`
		UPDATE tenants
		SET name = ?, slug = ?, description = ?, status = ?, auto_sync_enabled = ?, updated_at = ?
		WHERE id = ?
	`, name, slug, description, status, boolToInt(autoSyncEnabled), time.Now(), tenantID)
	return err
}

func ListAutoSyncTenantIDs() ([]int64, error) {
	rows, err := GetDB().Query(`
		SELECT id FROM tenants WHERE status = 'active' AND auto_sync_enabled = 1 ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenantIDs []int64
	for rows.Next() {
		var tenantID int64
		if err := rows.Scan(&tenantID); err != nil {
			return nil, err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	return tenantIDs, rows.Err()
}
