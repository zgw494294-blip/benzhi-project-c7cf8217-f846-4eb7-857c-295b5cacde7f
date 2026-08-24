package application

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type AuditEvent struct {
	ID         string          `json:"id"`
	VolumeID   string          `json:"volumeID"`
	Operation  string          `json:"operation"`
	Actor      string          `json:"actor"`
	Version    int64           `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Details    json.RawMessage `json:"details"`
}

type ImageObject struct {
	Key       string
	MediaType string
	Data      []byte
	SHA256    string
}

type Transaction interface {
	GetVolume(ctx context.Context, id string) (*domain.DigitizationVolume, error)
	SaveVolume(ctx context.Context, volume *domain.DigitizationVolume) error
	SaveImage(ctx context.Context, image ImageObject) error
	AppendAudit(ctx context.Context, event AuditEvent) error
}

type Store interface {
	Execute(ctx context.Context, idempotencyKey, operation, requestFingerprint string, fn func(Transaction) (json.RawMessage, error)) (json.RawMessage, bool, error)
	GetVolume(ctx context.Context, id string) (*domain.DigitizationVolume, error)
	ListVolumes(ctx context.Context) ([]domain.DigitizationVolume, error)
	ListAudit(ctx context.Context, volumeID string) ([]AuditEvent, error)
	OpenImage(ctx context.Context, key string) (io.ReadCloser, string, int64, error)
	Close() error
}
