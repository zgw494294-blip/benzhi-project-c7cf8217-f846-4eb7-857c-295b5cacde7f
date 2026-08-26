package canceled_idempotency_wait_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

func TestCanceledDuplicateCommandStopsWaitingForLeader(t *testing.T) {
	store := &blockingStore{entered: make(chan struct{}), release: make(chan struct{})}
	service := application.NewService(store)
	command := application.CreateVolume{Title: "并发幂等测试本", ShelfMark: "测·并一", Actor: "编目员"}

	leaderResult := make(chan error, 1)
	go func() {
		_, _, err := service.CreateVolume(context.Background(), "shared-key", command)
		leaderResult <- err
	}()
	<-store.entered

	base, cancel := context.WithCancel(context.Background())
	cancel()
	waiterContext := &doneObservedContext{Context: base, observed: make(chan struct{})}
	waiterResult := make(chan error, 1)
	go func() {
		_, _, err := service.CreateVolume(waiterContext, "shared-key", command)
		waiterResult <- err
	}()
	<-waiterContext.observed

	close(store.release)
	if err := <-leaderResult; err != nil {
		t.Fatalf("主请求失败: %v", err)
	}
	if err := <-waiterResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的重复请求应立即返回 context.Canceled，实际为 %v", err)
	}
}

type doneObservedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *doneObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

type blockingStore struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingStore) Execute(_ context.Context, _, _, _ string, fn func(application.Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error) {
	s.once.Do(func() { close(s.entered) })
	<-s.release
	raw, err := fn(memoryTransaction{})
	return raw, false, err
}

func (*blockingStore) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	return nil, errors.New("unexpected GetVolume")
}
func (*blockingStore) ListVolumes(context.Context) ([]domain.DigitizationVolume, error) {
	return nil, errors.New("unexpected ListVolumes")
}
func (*blockingStore) ListAudit(context.Context, string) ([]application.AuditEvent, error) {
	return nil, errors.New("unexpected ListAudit")
}
func (*blockingStore) OpenImage(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, errors.New("unexpected OpenImage")
}
func (*blockingStore) Close() error { return nil }

type memoryTransaction struct{}

func (memoryTransaction) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	return nil, errors.New("unexpected GetVolume")
}
func (memoryTransaction) SaveVolume(context.Context, *domain.DigitizationVolume) error { return nil }
func (memoryTransaction) SaveImage(context.Context, application.ImageObject) error     { return nil }
func (memoryTransaction) AppendAudit(context.Context, application.AuditEvent) error    { return nil }
