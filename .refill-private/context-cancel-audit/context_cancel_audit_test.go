package context_cancel_audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type cancelStore struct {
	cancel    context.CancelFunc
	committed bool
}

func (s *cancelStore) Execute(_ context.Context, _, _, _ string, fn func(application.Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error) {
	raw, err := fn(cancelTx{cancel: s.cancel})
	if err == nil {
		s.committed = true
	}
	return raw, false, err
}

func (*cancelStore) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	return nil, errors.New("unused")
}
func (*cancelStore) ListVolumes(context.Context) ([]domain.DigitizationVolume, error) {
	return nil, errors.New("unused")
}
func (*cancelStore) ListAudit(context.Context, string) ([]application.AuditEvent, error) {
	return nil, errors.New("unused")
}
func (*cancelStore) OpenImage(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, errors.New("unused")
}
func (*cancelStore) Close() error { return nil }

type cancelTx struct{ cancel context.CancelFunc }

func (cancelTx) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	return nil, errors.New("unused")
}
func (tx cancelTx) SaveVolume(context.Context, *domain.DigitizationVolume) error {
	tx.cancel()
	return nil
}
func (cancelTx) SaveImage(context.Context, application.ImageObject) error { return nil }
func (cancelTx) AppendAudit(ctx context.Context, _ application.AuditEvent) error {
	return ctx.Err()
}

func TestCancellationBetweenSaveAndAuditAbortsCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &cancelStore{cancel: cancel}
	service := application.NewService(store)

	_, _, err := service.CreateVolume(ctx, "cancel-key", application.CreateVolume{Title: "取消测试", ShelfMark: "甲一"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("请求在保存后取消，命令仍未返回 context.Canceled: %v", err)
	}
	if store.committed {
		t.Fatal("请求取消后命令仍被提交")
	}
}
