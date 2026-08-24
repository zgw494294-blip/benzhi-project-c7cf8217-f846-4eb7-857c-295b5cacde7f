package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
	_ "modernc.org/sqlite"
)

func TestSQLitePersistsAndReplaysCommand(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	command := application.CreateVolume{Title: "四库全书辑本", ShelfMark: "测试·一", Actor: "测试员"}
	first, replayed, err := service.CreateVolume(context.Background(), "test-key", command)
	if err != nil || replayed {
		t.Fatalf("首次建卷失败: %v", err)
	}
	second, replayed, err := service.CreateVolume(context.Background(), "test-key", command)
	if err != nil || !replayed || first.ID != second.ID {
		t.Fatalf("幂等重放失败: %v", err)
	}
	loaded, err := store.GetVolume(context.Background(), first.ID)
	if err != nil || loaded.Title != command.Title {
		t.Fatalf("读取卷失败: %v", err)
	}
	events, err := store.ListAudit(context.Background(), first.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("审计事件异常: %v %#v", err, events)
	}
}

func TestIdempotencyKeyRejectsDifferentPayload(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	ctx := context.Background()
	if _, _, err := service.CreateVolume(ctx, "one-key", application.CreateVolume{Title: "甲本", ShelfMark: "甲一"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.CreateVolume(ctx, "one-key", application.CreateVolume{Title: "乙本", ShelfMark: "乙一"}); application.ErrorCode(err) != domain.CodeConflict {
		t.Fatalf("不同请求载荷未返回冲突: %v", err)
	}
}

func TestMigratesSchemaVersionOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-one.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE schema_meta (version INTEGER NOT NULL); INSERT INTO schema_meta(version) VALUES(1); CREATE TABLE idempotency_results (idempotency_key TEXT PRIMARY KEY, operation TEXT NOT NULL, result_json BLOB NOT NULL, created_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow(`SELECT version FROM schema_meta`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatalf("schemaVersion 未升级: %d, %v", version, err)
	}
	var fingerprint string
	err = store.db.QueryRow(`SELECT request_fingerprint FROM idempotency_results LIMIT 1`).Scan(&fingerprint)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("request_fingerprint 列不可用: %v", err)
	}
}
