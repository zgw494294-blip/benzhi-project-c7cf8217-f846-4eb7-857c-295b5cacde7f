package migration_atomicity_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
	_ "modernc.org/sqlite"
)

func TestFailedMigrationLeavesVersionOneSchemaUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE schema_meta (version INTEGER NOT NULL);
		INSERT INTO schema_meta(version) VALUES(1);
		CREATE TABLE idempotency_results (
			idempotency_key TEXT PRIMARY KEY, operation TEXT NOT NULL,
			result_json BLOB NOT NULL, created_at TEXT NOT NULL
		);
		CREATE TRIGGER reject_version_update BEFORE UPDATE ON schema_meta
		BEGIN SELECT RAISE(ABORT, 'forced version write failure'); END;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if opened, err := storage.Open(path); err == nil {
		opened.Close()
		t.Fatal("受控版本写入失败时迁移意外成功")
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(idempotency_results)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "request_fingerprint" {
			t.Fatal("迁移失败后 request_fingerprint 列仍被永久写入，schemaVersion 却仍为 1")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
