package metadata_audit_cross_talk

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

func TestConcurrentMetadataUpdatesKeepAuditOwnership(t *testing.T) {
	store := newControlledStore()
	service := application.NewService(store)
	firstResult := make(chan error, 1)
	secondResult := make(chan error, 1)

	go func() {
		_, _, err := service.UpdateMetadata(context.Background(), "key-first", "vol-first", application.UpdateMetadata{
			ExpectedVersion: 1, Title: "甲卷修订", ShelfMark: "A-1", Actor: "甲校勘员",
		})
		firstResult <- err
	}()
	<-store.firstSaved

	go func() {
		_, _, err := service.UpdateMetadata(context.Background(), "key-second", "vol-second", application.UpdateMetadata{
			ExpectedVersion: 1, Title: "乙卷修订", ShelfMark: "B-1", Actor: "乙校勘员",
		})
		secondResult <- err
	}()
	<-store.secondSaved

	close(store.releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("第一个元数据更新失败: %v", err)
	}
	close(store.releaseSecond)
	if err := <-secondResult; err != nil {
		t.Fatalf("第二个元数据更新失败: %v", err)
	}

	firstAudit := store.auditFor("vol-first")
	if firstAudit.VolumeID != "vol-first" || firstAudit.Actor != "甲校勘员" {
		t.Fatalf("第一个事务的审计归属被并发请求污染: volumeID=%q actor=%q", firstAudit.VolumeID, firstAudit.Actor)
	}
}

type controlledStore struct {
	volumes       map[string]*domain.DigitizationVolume
	firstSaved    chan struct{}
	secondSaved   chan struct{}
	releaseFirst  chan struct{}
	releaseSecond chan struct{}
	mu            sync.Mutex
	audits        map[string]application.AuditEvent
}

func newControlledStore() *controlledStore {
	now := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC)
	return &controlledStore{
		volumes: map[string]*domain.DigitizationVolume{
			"vol-first":  {ID: "vol-first", Title: "甲卷", ShelfMark: "A-1", State: domain.StateDraft, Version: 1, CreatedAt: now, UpdatedAt: now},
			"vol-second": {ID: "vol-second", Title: "乙卷", ShelfMark: "B-1", State: domain.StateDraft, Version: 1, CreatedAt: now, UpdatedAt: now},
		},
		firstSaved: make(chan struct{}), secondSaved: make(chan struct{}),
		releaseFirst: make(chan struct{}), releaseSecond: make(chan struct{}),
		audits: make(map[string]application.AuditEvent),
	}
}

func (s *controlledStore) Execute(ctx context.Context, key, operation, fingerprint string, fn func(application.Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error) {
	tx := &controlledTransaction{store: s}
	raw, err := fn(tx)
	return raw, false, err
}

func (s *controlledStore) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	panic("unexpected direct GetVolume")
}

func (s *controlledStore) ListVolumes(context.Context) ([]domain.DigitizationVolume, error) {
	panic("unexpected ListVolumes")
}

func (s *controlledStore) ListAudit(context.Context, string) ([]application.AuditEvent, error) {
	panic("unexpected ListAudit")
}

func (s *controlledStore) OpenImage(context.Context, string) (io.ReadCloser, string, int64, error) {
	panic("unexpected OpenImage")
}

func (s *controlledStore) Close() error { return nil }

func (s *controlledStore) auditFor(volumeID string) application.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.audits[volumeID]
}

type controlledTransaction struct {
	store    *controlledStore
	volumeID string
}

func (tx *controlledTransaction) GetVolume(_ context.Context, id string) (*domain.DigitizationVolume, error) {
	volume := *tx.store.volumes[id]
	tx.volumeID = id
	return &volume, nil
}

func (tx *controlledTransaction) SaveVolume(_ context.Context, volume *domain.DigitizationVolume) error {
	switch volume.ID {
	case "vol-first":
		close(tx.store.firstSaved)
		<-tx.store.releaseFirst
	case "vol-second":
		close(tx.store.secondSaved)
		<-tx.store.releaseSecond
	}
	return nil
}

func (tx *controlledTransaction) SaveImage(context.Context, application.ImageObject) error {
	panic("unexpected SaveImage")
}

func (tx *controlledTransaction) AppendAudit(_ context.Context, event application.AuditEvent) error {
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	tx.store.audits[tx.volumeID] = event
	return nil
}
