package storage

import (
	"context"
	"fmt"
)

const schemaVersion = 2

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS volumes (
        id TEXT PRIMARY KEY, title TEXT NOT NULL, edition_note TEXT NOT NULL, shelf_mark TEXT NOT NULL,
        state TEXT NOT NULL, version INTEGER NOT NULL, page_order_json BLOB NOT NULL,
        latest_check_id TEXT NOT NULL, frozen_digest TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS pages (
        id TEXT PRIMARY KEY, volume_id TEXT NOT NULL REFERENCES volumes(id) ON DELETE CASCADE,
        folio_label TEXT NOT NULL, sequence INTEGER NOT NULL, image_object_key TEXT NOT NULL,
        media_type TEXT NOT NULL, byte_size INTEGER NOT NULL, sha256 TEXT NOT NULL,
        width INTEGER NOT NULL, height INTEGER NOT NULL, transcription TEXT NOT NULL, revision INTEGER NOT NULL
    )`,
	`CREATE INDEX IF NOT EXISTS pages_volume_sequence ON pages(volume_id, sequence)`,
	`CREATE TABLE IF NOT EXISTS findings (
        id TEXT PRIMARY KEY, volume_id TEXT NOT NULL REFERENCES volumes(id) ON DELETE CASCADE,
        page_id TEXT NOT NULL, location TEXT NOT NULL, category TEXT NOT NULL, observed_text TEXT NOT NULL,
        proposed_text TEXT NOT NULL, status TEXT NOT NULL, resolution TEXT NOT NULL,
        resolved_by TEXT NOT NULL, resolved_at TEXT
    )`,
	`CREATE TABLE IF NOT EXISTS check_runs (
        id TEXT PRIMARY KEY, volume_id TEXT NOT NULL REFERENCES volumes(id) ON DELETE CASCADE,
        run_number INTEGER NOT NULL, status TEXT NOT NULL, checked_version INTEGER NOT NULL,
        violations_json BLOB NOT NULL, started_at TEXT NOT NULL, completed_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS manifests (
        id TEXT PRIMARY KEY, volume_id TEXT NOT NULL UNIQUE REFERENCES volumes(id) ON DELETE CASCADE,
        manifest_number TEXT NOT NULL UNIQUE, frozen_digest TEXT NOT NULL, page_digests_json BLOB NOT NULL,
        reviewer TEXT NOT NULL, review_note TEXT NOT NULL, issued_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS image_objects (
        object_key TEXT PRIMARY KEY, media_type TEXT NOT NULL, sha256 TEXT NOT NULL, byte_size INTEGER NOT NULL, data BLOB NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS idempotency_results (
		idempotency_key TEXT PRIMARY KEY, operation TEXT NOT NULL, request_fingerprint TEXT NOT NULL,
		result_json BLOB NOT NULL, created_at TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS audit_events (
        sequence INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, volume_id TEXT NOT NULL,
        operation TEXT NOT NULL, actor TEXT NOT NULL, version INTEGER NOT NULL, occurred_at TEXT NOT NULL, details_json BLOB NOT NULL
    )`,
	`CREATE INDEX IF NOT EXISTS audit_volume_sequence ON audit_events(volume_id, sequence)`,
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return err
	}
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行数据库迁移: %w", err)
		}
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := s.db.ExecContext(ctx, `INSERT INTO schema_meta(version) VALUES(?)`, schemaVersion)
		return err
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version == 1 {
		// 将 schema 修改与版本元数据写入置于同一事务内，任一步失败都回滚，
		// 避免残留新列而版本仍为 1，导致下次启动因重复添加列而迁移失败。
		if err := s.upgradeToV2(ctx); err != nil {
			return err
		}
		version = schemaVersion
	}
	if version != schemaVersion {
		return fmt.Errorf("不支持的 schemaVersion：%d", version)
	}
	return nil
}

// upgradeToV2 原子地把 schema 版本 1 升级到版本 2。
//
// 使用单个事务包裹列添加（幂等）和版本元数据更新；任一步失败都会回滚，
// 使数据库停留在可重试的原始状态。若升级因残留新列而以“duplicate column”
// 失败（由此前崩溃的写入引起），则检测列已存在并跳过添加，仅推进版本，
// 从而修复已损坏的库。
func (s *SQLiteStore) upgradeToV2(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始 schema 升级事务: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	columnExists, err := columnExists(ctx, tx, "idempotency_results", "request_fingerprint")
	if err != nil {
		return fmt.Errorf("升级 schemaVersion 1: %w", err)
	}
	if !columnExists {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE idempotency_results ADD COLUMN request_fingerprint TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("升级 schemaVersion 1: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_meta SET version = ?`, schemaVersion); err != nil {
		return fmt.Errorf("记录 schemaVersion 2: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交 schema 升级事务: %w", err)
	}
	return nil
}

// columnExists 通过 pragma_table_info 检查指定表是否存在某列。
func columnExists(ctx context.Context, q queryer, table, column string) (bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT 1 FROM pragma_table_info(?) WHERE name = ?`, table, column)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		return true, nil
	}
	return false, rows.Err()
}

func (s *SQLiteStore) consistencyCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result); err != nil {
		return fmt.Errorf("SQLite 一致性检查失败: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("SQLite 一致性检查未通过: %s", result)
	}
	var broken int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pages p LEFT JOIN volumes v ON v.id=p.volume_id WHERE v.id IS NULL OR p.sha256 <> p.image_object_key`).Scan(&broken)
	if err != nil {
		return err
	}
	if broken > 0 {
		return fmt.Errorf("发现 %d 条孤立或摘要不一致的页面记录", broken)
	}
	return nil
}
