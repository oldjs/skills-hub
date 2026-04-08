package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	database *sql.DB
	once     sync.Once
)

func Init(dbPath string) error {
	var err error
	once.Do(func() {
		database, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return
		}

		if err = setupPragmas(); err != nil {
			return
		}

		if err = createCoreTables(); err != nil {
			return
		}

		if err = migrateLegacyData(); err != nil {
			return
		}

		if err = migrateAddContentColumn(); err != nil {
			return
		}

		if err = migrateAddAuthorColumn(); err != nil {
			return
		}

		if err = migrateAddDownloadCountColumn(); err != nil {
			return
		}

		if err = migrateAddSkillReviewColumns(); err != nil {
			return
		}

		if err = migrateAddSubAdminColumn(); err != nil {
			return
		}

		if err = migrateAddUserProfileColumns(); err != nil {
			return
		}

		if err = migrateAddCommentParentID(); err != nil {
			return
		}

		if err = migrateCreateNotificationsTable(); err != nil {
			return
		}

		if err = migrateCreateBookmarksTable(); err != nil {
			return
		}

		if err = createIndexes(); err != nil {
			return
		}

		// FTS5 全文搜索索引（失败不阻塞启动）
		_ = initFTS()

		log.Println("Database initialized successfully")
	})
	return err
}

func setupPragmas() error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}

	for _, pragma := range pragmas {
		if _, err := database.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}

func createCoreTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
			auto_sync_enabled INTEGER NOT NULL DEFAULT 1 CHECK (auto_sync_enabled IN (0, 1)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
			is_platform_admin INTEGER NOT NULL DEFAULT 0 CHECK (is_platform_admin IN (0, 1)),
			is_sub_admin INTEGER NOT NULL DEFAULT 0 CHECK (is_sub_admin IN (0, 1)),
			last_tenant_id INTEGER,
			last_login_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (last_tenant_id) REFERENCES tenants(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tenant_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, user_id),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS tenant_invites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			email TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			slug TEXT NOT NULL,
			display_name TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			score REAL NOT NULL DEFAULT 0,
			source_updated_at INTEGER NOT NULL DEFAULT 0,
			version TEXT NOT NULL DEFAULT '',
			categories TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			download_count INTEGER NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT 'clawhub',
			review_status TEXT NOT NULL DEFAULT 'approved',
			review_note TEXT NOT NULL DEFAULT '',
			reviewed_by INTEGER,
			reviewed_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, slug),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sync_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			keywords TEXT NOT NULL DEFAULT '',
			count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'failed')),
			message TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`,
		// 评分表，每个用户每个skill只能评一次
		`CREATE TABLE IF NOT EXISTS skill_ratings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			skill_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			score INTEGER NOT NULL CHECK (score >= 1 AND score <= 5),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, skill_id, user_id),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		// 评论表
		`CREATE TABLE IF NOT EXISTS skill_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			skill_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS admin_action_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_user_id INTEGER NOT NULL,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '',
			target_id INTEGER NOT NULL DEFAULT 0,
			details TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (admin_user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_used_at DATETIME,
			revoked_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
	}

	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}

func createIndexes() error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)`,
		`CREATE INDEX IF NOT EXISTS idx_users_last_tenant_id ON users(last_tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_members_user_id ON tenant_members(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant_status ON tenant_members(tenant_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_invites_email_status ON tenant_invites(email, status)`,
		`CREATE INDEX IF NOT EXISTS idx_tenant_invites_tenant_email_status ON tenant_invites(tenant_id, email, status)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_invites_pending_unique ON tenant_invites(tenant_id, email) WHERE status = 'pending'`,
		`CREATE INDEX IF NOT EXISTS idx_skills_tenant_display_name ON skills(tenant_id, display_name)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_tenant_score_name ON skills(tenant_id, score DESC, display_name ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_tenant_categories ON skills(tenant_id, categories)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_tenant_review_status ON skills(tenant_id, review_status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_log_tenant_synced_at ON sync_log(tenant_id, synced_at DESC)`,
		// 评分表索引
		`CREATE INDEX IF NOT EXISTS idx_skill_ratings_skill ON skill_ratings(tenant_id, skill_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_ratings_user ON skill_ratings(user_id)`,
		// 评论表索引
		`CREATE INDEX IF NOT EXISTS idx_skill_comments_skill ON skill_comments(tenant_id, skill_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_comments_created ON skill_comments(skill_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_action_logs_created ON admin_action_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_admin_action_logs_admin_user_id ON admin_action_logs(admin_user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id_created_at ON api_keys(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_revoked_at ON api_keys(revoked_at)`,
		// Profile 页用的索引
		`CREATE INDEX IF NOT EXISTS idx_skill_comments_user_id ON skill_comments(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_ratings_user_created ON skill_ratings(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_comments_parent ON skill_comments(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_bookmarks_user ON skill_bookmarks(user_id, tenant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, is_read, created_at DESC)`,
	}

	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyData() error {
	hasTenantID, err := tableHasColumn("skills", "tenant_id")
	if err != nil {
		return err
	}
	if hasTenantID {
		return nil
	}

	legacySkills, err := tableExists("skills")
	if err != nil {
		return err
	}
	legacySyncLog, err := tableExists("sync_log")
	if err != nil {
		return err
	}
	if !legacySkills && !legacySyncLog {
		return nil
	}

	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	defaultTenantID, err := ensureTenantTx(tx, "default", "Default Workspace", "历史数据默认租户")
	if err != nil {
		return err
	}

	if legacySkills {
		if _, err := tx.Exec(`ALTER TABLE skills RENAME TO skills_legacy`); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE TABLE skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			slug TEXT NOT NULL,
			display_name TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			score REAL NOT NULL DEFAULT 0,
			source_updated_at INTEGER NOT NULL DEFAULT 0,
			version TEXT NOT NULL DEFAULT '',
			categories TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			download_count INTEGER NOT NULL DEFAULT 0,
			source TEXT NOT NULL DEFAULT 'clawhub',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, slug),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO skills (tenant_id, slug, display_name, summary, score, source_updated_at, version, categories, author, download_count, source, created_at, updated_at)
			SELECT ?, slug, display_name, COALESCE(summary, ''), COALESCE(score, 0), COALESCE(updated_at, 0), COALESCE(version, ''), COALESCE(categories, ''), 'ClawHub', 0, 'clawhub', COALESCE(created_at, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP
			FROM skills_legacy`, defaultTenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE skills_legacy`); err != nil {
			return err
		}
	}

	if legacySyncLog {
		if _, err := tx.Exec(`ALTER TABLE sync_log RENAME TO sync_log_legacy`); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE TABLE sync_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			keywords TEXT NOT NULL DEFAULT '',
			count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'failed')),
			message TEXT NOT NULL DEFAULT '',
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO sync_log (tenant_id, synced_at, keywords, count, status, message)
			SELECT ?, COALESCE(synced_at, CURRENT_TIMESTAMP), COALESCE(keywords, ''), COALESCE(count, 0), 'success', ''
			FROM sync_log_legacy`, defaultTenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE sync_log_legacy`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func ensureTenantTx(tx *sql.Tx, slug, name, description string) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM tenants WHERE slug = ?`, slug).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := tx.Exec(`INSERT INTO tenants (slug, name, description) VALUES (?, ?, ?)`, slug, name, description)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func tableExists(name string) (bool, error) {
	row := database.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`, name)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableHasColumn(tableName, columnName string) (bool, error) {
	rows, err := database.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}

// 给 skills 表加 content 字段，存 SKILL.md 内容
func migrateAddContentColumn() error {
	hasContent, err := tableHasColumn("skills", "content")
	if err != nil {
		return err
	}
	if hasContent {
		return nil
	}

	_, err = database.Exec(`ALTER TABLE skills ADD COLUMN content TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		log.Printf("添加 content 列失败（可能已存在）: %v", err)
	}
	return nil
}

func migrateAddAuthorColumn() error {
	hasAuthor, err := tableHasColumn("skills", "author")
	if err != nil {
		return err
	}
	if !hasAuthor {
		if _, err := database.Exec(`ALTER TABLE skills ADD COLUMN author TEXT NOT NULL DEFAULT ''`); err != nil {
			log.Printf("添加 author 列失败（可能已存在）: %v", err)
		}
	}

	if _, err := database.Exec(`UPDATE skills SET author = 'ClawHub' WHERE source != 'upload' AND author = ''`); err != nil {
		log.Printf("回填 author 失败: %v", err)
	}
	return nil
}

func migrateAddDownloadCountColumn() error {
	hasDownloadCount, err := tableHasColumn("skills", "download_count")
	if err != nil {
		return err
	}
	if hasDownloadCount {
		return nil
	}

	if _, err := database.Exec(`ALTER TABLE skills ADD COLUMN download_count INTEGER NOT NULL DEFAULT 0`); err != nil {
		log.Printf("添加 download_count 列失败（可能已存在）: %v", err)
	}
	return nil
}

func migrateAddSkillReviewColumns() error {
	columns := []struct {
		name        string
		statement   string
		backfillSQL string
	}{
		{name: "review_status", statement: `ALTER TABLE skills ADD COLUMN review_status TEXT NOT NULL DEFAULT 'approved'`, backfillSQL: `UPDATE skills SET review_status = 'approved' WHERE COALESCE(review_status, '') = ''`},
		{name: "review_note", statement: `ALTER TABLE skills ADD COLUMN review_note TEXT NOT NULL DEFAULT ''`},
		{name: "reviewed_by", statement: `ALTER TABLE skills ADD COLUMN reviewed_by INTEGER`},
		{name: "reviewed_at", statement: `ALTER TABLE skills ADD COLUMN reviewed_at DATETIME`},
	}

	for _, column := range columns {
		hasColumn, err := tableHasColumn("skills", column.name)
		if err != nil {
			return err
		}
		if !hasColumn {
			if _, err := database.Exec(column.statement); err != nil {
				log.Printf("添加 %s 列失败（可能已存在）: %v", column.name, err)
			}
		}
		if column.backfillSQL != "" {
			if _, err := database.Exec(column.backfillSQL); err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateAddSubAdminColumn() error {
	hasSubAdmin, err := tableHasColumn("users", "is_sub_admin")
	if err != nil {
		return err
	}
	if hasSubAdmin {
		return nil
	}

	if _, err := database.Exec(`ALTER TABLE users ADD COLUMN is_sub_admin INTEGER NOT NULL DEFAULT 0`); err != nil {
		log.Printf("添加 is_sub_admin 列失败（可能已存在）: %v", err)
	}
	return nil
}

// 通知表
func migrateCreateNotificationsTable() error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			link TEXT NOT NULL DEFAULT '',
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	return err
}

// 收藏表
func migrateCreateBookmarksTable() error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS skill_bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			skill_id INTEGER NOT NULL,
			tenant_id INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, skill_id, tenant_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)
	`)
	return err
}

// 给评论表加 parent_id 字段（楼中楼回复）
func migrateAddCommentParentID() error {
	hasParentID, err := tableHasColumn("skill_comments", "parent_id")
	if err != nil {
		return err
	}
	if hasParentID {
		return nil
	}
	if _, err := database.Exec(`ALTER TABLE skill_comments ADD COLUMN parent_id INTEGER DEFAULT NULL`); err != nil {
		log.Printf("添加 parent_id 列失败（可能已存在）: %v", err)
	}
	return nil
}

// 给 users 表加 bio 字段（个人简介）
func migrateAddUserProfileColumns() error {
	hasBio, err := tableHasColumn("users", "bio")
	if err != nil {
		return err
	}
	if hasBio {
		return nil
	}
	if _, err := database.Exec(`ALTER TABLE users ADD COLUMN bio TEXT NOT NULL DEFAULT ''`); err != nil {
		log.Printf("添加 bio 列失败（可能已存在）: %v", err)
	}
	return nil
}

func GetDB() *sql.DB {
	return database
}

func Close() {
	if database != nil {
		_ = database.Close()
	}
}
