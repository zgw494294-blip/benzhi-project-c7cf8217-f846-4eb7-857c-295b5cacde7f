package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db       *sql.DB
	imageMu  sync.Mutex
	imageKey string
	image    *cachedImage
}

type cachedImage struct {
	data  []byte
	media string
}

type cachedImageReader struct {
	image  *cachedImage
	offset int
}

func (r *cachedImageReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.image.data) {
		return 0, io.EOF
	}
	n := copy(p, r.image.data[r.offset:])
	r.offset += n
	return n, nil
}

func (r *cachedImageReader) Close() error { return nil }

func Open(dataSource string) (*SQLiteStore, error) {
	if strings.TrimSpace(dataSource) == "" {
		dataSource = "collation.db"
	}
	db, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	if dataSource == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(8)
	}
	store := &SQLiteStore{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.consistencyCheck(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) Execute(ctx context.Context, key, operation, requestFingerprint string, fn func(application.Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("开始事务: %w", err)
	}
	defer tx.Rollback()
	var storedOperation string
	var storedFingerprint string
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT operation, request_fingerprint, result_json FROM idempotency_results WHERE idempotency_key = ?`, key).Scan(&storedOperation, &storedFingerprint, &raw)
	if err == nil {
		if storedOperation != operation || (storedFingerprint != "" && storedFingerprint != requestFingerprint) {
			return nil, false, domain.NewRuleError(domain.CodeConflict, "Idempotency-Key 已用于其他请求")
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return json.RawMessage(raw), true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	result, err := fn(&repository{tx: tx})
	if err != nil {
		return nil, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_results(idempotency_key, operation, request_fingerprint, result_json, created_at) VALUES(?,?,?,?,?)`, key, operation, requestFingerprint, []byte(result), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, false, fmt.Errorf("保存幂等结果: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("提交事务: %w", err)
	}
	return result, false, nil
}

func (s *SQLiteStore) GetVolume(ctx context.Context, id string) (*domain.DigitizationVolume, error) {
	return loadVolume(ctx, s.db, id)
}

func (s *SQLiteStore) ListVolumes(ctx context.Context) ([]domain.DigitizationVolume, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM volumes ORDER BY updated_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	volumes := make([]domain.DigitizationVolume, 0, len(ids))
	for _, id := range ids {
		volume, err := loadVolume(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, *volume)
	}
	return volumes, nil
}

func (s *SQLiteStore) ListAudit(ctx context.Context, volumeID string) ([]application.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, volume_id, operation, actor, version, occurred_at, details_json FROM audit_events WHERE volume_id = ? ORDER BY sequence`, volumeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]application.AuditEvent, 0)
	for rows.Next() {
		var event application.AuditEvent
		var occurred string
		var raw []byte
		if err := rows.Scan(&event.ID, &event.VolumeID, &event.Operation, &event.Actor, &event.Version, &occurred, &raw); err != nil {
			return nil, err
		}
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		event.Details = raw
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) OpenImage(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	s.imageMu.Lock()
	if s.imageKey == key && s.image != nil {
		image := s.image
		s.imageMu.Unlock()
		return &cachedImageReader{image: image}, image.media, int64(len(image.data)), nil
	}
	s.imageMu.Unlock()

	var media string
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT media_type, data FROM image_objects WHERE object_key = ?`, key).Scan(&media, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", 0, domain.NewRuleError(domain.CodeNotFound, "图像对象不存在")
	}
	if err != nil {
		return nil, "", 0, err
	}
	if domain.HashBytes(data) != key {
		return nil, "", 0, domain.NewRuleError(domain.CodeConflict, "图像对象摘要校验失败")
	}
	image := &cachedImage{data: data, media: media}
	s.imageMu.Lock()
	s.imageKey = key
	s.image = image
	s.imageMu.Unlock()
	return &cachedImageReader{image: image}, media, int64(len(data)), nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
