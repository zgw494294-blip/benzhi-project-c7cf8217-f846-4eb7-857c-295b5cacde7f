package workbench_cache_alias_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type fixedStore struct {
	volume *domain.DigitizationVolume
	reads  int
}

func (s *fixedStore) GetVolume(context.Context, string) (*domain.DigitizationVolume, error) {
	s.reads++
	return s.volume, nil
}

func (s *fixedStore) Execute(context.Context, string, string, string, func(application.Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error) {
	panic("unexpected command")
}

func (s *fixedStore) ListVolumes(context.Context) ([]domain.DigitizationVolume, error) {
	return nil, nil
}

func (s *fixedStore) ListAudit(context.Context, string) ([]application.AuditEvent, error) {
	return nil, nil
}

func (s *fixedStore) OpenImage(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, nil
}

func (s *fixedStore) Close() error { return nil }

func TestCachedWorkbenchDoesNotExposeMutablePageAliases(t *testing.T) {
	store := &fixedStore{volume: &domain.DigitizationVolume{
		ID: "vol-archived", State: domain.StateAccessioned,
		PageOrder: []string{"page-1"},
		Pages:     []domain.FacsimilePage{{ID: "page-1", VolumeID: "vol-archived", Transcription: "天地玄黄"}},
	}}
	service := application.NewService(store)

	first, err := service.GetWorkbench(context.Background(), store.volume.ID)
	if err != nil {
		t.Fatal(err)
	}
	first.OrderedPages[0].Transcription = "被调用方污染"

	second, err := service.GetWorkbench(context.Background(), store.volume.ID)
	if err != nil {
		t.Fatal(err)
	}
	if store.reads != 1 {
		t.Fatalf("未命中已入藏卷缓存: reads=%d", store.reads)
	}
	if second.OrderedPages[0].Transcription != "天地玄黄" {
		t.Fatalf("缓存工作台复用可变切片，后续请求读到污染转录: %q", second.OrderedPages[0].Transcription)
	}
}
