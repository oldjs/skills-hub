package db

import (
	"log"
	"strings"
)

// 创建 FTS5 虚拟表并填充数据
func initFTS() error {
	// FTS5 虚拟表，用 unicode61 分词器（中英文都能用）
	_, err := database.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
			display_name, summary, categories, content,
			tokenize='unicode61'
		)
	`)
	if err != nil {
		// FTS5 不可用的话不阻塞启动，退回 LIKE 查询
		log.Printf("FTS5 初始化失败（将使用 LIKE 回退）: %v", err)
		return nil
	}

	// 检查是否需要重建索引（空表时全量填充）
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM skills_fts`).Scan(&count); err != nil {
		log.Printf("FTS5 计数失败: %v", err)
		return nil
	}
	if count == 0 {
		log.Println("FTS5 索引为空，执行全量填充...")
		rebuildFTSIndex()
	}
	return nil
}

// 全量重建 FTS 索引
func rebuildFTSIndex() {
	_, _ = database.Exec(`DELETE FROM skills_fts`)
	result, err := database.Exec(`
		INSERT INTO skills_fts(rowid, display_name, summary, categories, content)
		SELECT id, display_name, summary, categories, COALESCE(content, '')
		FROM skills
	`)
	if err != nil {
		log.Printf("FTS5 全量填充失败: %v", err)
		return
	}
	rows, _ := result.RowsAffected()
	log.Printf("FTS5 索引已重建，共 %d 条", rows)
}

// 单条 skill 写入/更新 FTS 索引
func SyncSkillToFTS(skillID int64) {
	if !ftsAvailable() {
		return
	}
	// 先删后插，确保幂等
	_, _ = database.Exec(`DELETE FROM skills_fts WHERE rowid = ?`, skillID)
	_, err := database.Exec(`
		INSERT INTO skills_fts(rowid, display_name, summary, categories, content)
		SELECT id, display_name, summary, categories, COALESCE(content, '')
		FROM skills WHERE id = ?
	`, skillID)
	if err != nil {
		log.Printf("FTS5 同步 skill %d 失败: %v", skillID, err)
	}
}

// 删除 FTS 索引条目
func RemoveSkillFromFTS(skillID int64) {
	if !ftsAvailable() {
		return
	}
	_, _ = database.Exec(`DELETE FROM skills_fts WHERE rowid = ?`, skillID)
}

// 检查 FTS5 表是否可用
func ftsAvailable() bool {
	var n int
	err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='skills_fts'`).Scan(&n)
	return err == nil && n > 0
}

// FTS5 全文搜索，返回匹配的 skill ID 列表（按相关度排序）
// 如果 FTS 不可用或查询为空，返回 nil 表示应回退到 LIKE
func SearchSkillIDsByFTS(query string) []int64 {
	if !ftsAvailable() || query == "" {
		return nil
	}

	// 把用户输入转成 FTS5 查询表达式
	// 多个词用 AND 连接，每个词加 * 做前缀匹配
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil
	}

	rows, err := database.Query(`
		SELECT rowid FROM skills_fts WHERE skills_fts MATCH ?
		ORDER BY bm25(skills_fts) LIMIT 500
	`, ftsQuery)
	if err != nil {
		log.Printf("FTS5 查询失败，回退 LIKE: %v", err)
		return nil
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// 把用户输入转成 FTS5 MATCH 表达式
// "browser agent" -> "browser* AND agent*"
// 支持中文（unicode61 会按字符拆分）
func buildFTSQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	words := strings.Fields(input)
	var terms []string
	for _, w := range words {
		// 过滤掉太短的纯 ASCII 词
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		// FTS5 安全转义：双引号包裹 + 加 * 做前缀匹配
		escaped := strings.ReplaceAll(w, "\"", "")
		if escaped == "" {
			continue
		}
		terms = append(terms, "\""+escaped+"\"*")
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " AND ")
}
