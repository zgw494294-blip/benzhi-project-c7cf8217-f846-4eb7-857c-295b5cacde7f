package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type Service struct {
	store Store
	now   func() time.Time
	id    func(string) string
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }, id: newID}
}

func newID(prefix string) string {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(bytes)
}

func runCommand[T any](ctx context.Context, service *Service, key, operation, scope string, command any, fn func(Transaction) (T, error)) (T, bool, error) {
	var zero T
	key = strings.TrimSpace(key)
	if key == "" {
		return zero, false, domain.NewRuleError(domain.CodeInvalid, "Idempotency-Key 不能为空")
	}
	fingerprintSource, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Scope     string `json:"scope"`
		Command   any    `json:"command"`
	}{Operation: operation, Scope: scope, Command: command})
	if err != nil {
		return zero, false, fmt.Errorf("生成幂等请求指纹: %w", err)
	}
	fingerprint := domain.HashBytes(fingerprintSource)
	raw, replayed, err := service.store.Execute(ctx, key, operation, fingerprint, func(tx Transaction) (json.RawMessage, error) {
		result, commandErr := fn(tx)
		if commandErr != nil {
			return nil, commandErr
		}
		return json.Marshal(result)
	})
	if err != nil {
		return zero, false, err
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return zero, false, fmt.Errorf("读取幂等命令结果: %w", err)
	}
	return result, replayed, nil
}

func requireVersion(volume *domain.DigitizationVolume, expected int64) error {
	if expected <= 0 {
		return domain.NewRuleError(domain.CodeInvalid, "expectedVersion 必须大于零")
	}
	if volume.Version != expected {
		return domain.NewRuleError(domain.CodeConflict, "版本冲突：当前版本为 %d，提交版本为 %d", volume.Version, expected)
	}
	return nil
}

func (s *Service) audit(tx Transaction, volume *domain.DigitizationVolume, operation, actor string, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return tx.AppendAudit(context.Background(), AuditEvent{
		ID: s.id("evt"), VolumeID: volume.ID, Operation: operation,
		Actor: strings.TrimSpace(actor), Version: volume.Version,
		OccurredAt: s.now(), Details: raw,
	})
}

func bump(volume *domain.DigitizationVolume, now time.Time) {
	volume.Version++
	volume.UpdatedAt = now
}

func markEdited(volume *domain.DigitizationVolume) error {
	if volume.State == domain.StateReadyForReview {
		return volume.Transition(domain.StateTranscribing)
	}
	return nil
}

func hasCurrentPassedCheck(volume *domain.DigitizationVolume) bool {
	if volume.LatestCheckID == "" || len(volume.Checks) == 0 {
		return false
	}
	latest := volume.Checks[len(volume.Checks)-1]
	return latest.ID == volume.LatestCheckID && latest.Status == "Passed" && latest.CheckedVersion == volume.Version-1
}

func pageByID(volume *domain.DigitizationVolume, id string) (*domain.FacsimilePage, error) {
	for index := range volume.Pages {
		if volume.Pages[index].ID == id {
			return &volume.Pages[index], nil
		}
	}
	return nil, domain.NewRuleError(domain.CodeNotFound, "页面不存在")
}

func findingByID(volume *domain.DigitizationVolume, id string) (*domain.CollationFinding, error) {
	for index := range volume.Findings {
		if volume.Findings[index].ID == id {
			return &volume.Findings[index], nil
		}
	}
	return nil, domain.NewRuleError(domain.CodeNotFound, "校勘问题不存在")
}

func ErrorCode(err error) domain.ErrorCode {
	var rule *domain.RuleError
	if errors.As(err, &rule) {
		return rule.Code
	}
	return "internal"
}
