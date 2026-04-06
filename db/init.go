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

		if err = createIndexes(); err != nil {
			return
		}

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
			source TEXT NOT NULL DEFAULT 'clawhub',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, slug),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
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
		`CREATE INDEX IF NOT EXISTS idx_sync_log_tenant_synced_at ON sync_log(tenant_id, synced_at DESC)`,
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
			source TEXT NOT NULL DEFAULT 'clawhub',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (tenant_id, slug),
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
		)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO skills (tenant_id, slug, display_name, summary, score, source_updated_at, version, categories, source, created_at, updated_at)
			SELECT ?, slug, display_name, COALESCE(summary, ''), COALESCE(score, 0), COALESCE(updated_at, 0), COALESCE(version, ''), COALESCE(categories, ''), 'clawhub', COALESCE(created_at, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP
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

func GetDB() *sql.DB {
	return database
}

func Close() {
	if database != nil {
		_ = database.Close()
	}
}
