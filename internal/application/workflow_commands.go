package application

import (
	"context"
	"fmt"
	"strings"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type VersionedCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
}

type AccessionCommand struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Reviewer        string `json:"reviewer"`
	ReviewNote      string `json:"reviewNote"`
}

func (s *Service) RunIntegrityCheck(ctx context.Context, key, volumeID string, command VersionedCommand) (*domain.DigitizationVolume, bool, error) {
	return runCommand(ctx, s, key, "integrity.run", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		volume, err := tx.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		if err := requireVersion(volume, command.ExpectedVersion); err != nil {
			return nil, err
		}
		if volume.State != domain.StateTranscribing && volume.State != domain.StateNeedsCorrection && volume.State != domain.StateReadyForReview {
			return nil, domain.NewRuleError(domain.CodeForbidden, "当前状态不能执行完整性检查")
		}
		if err := volume.Transition(domain.StateChecking); err != nil {
			return nil, err
		}
		started := s.now()
		violations := domain.CheckIntegrity(volume)
		target := domain.StateReadyForReview
		status := "Passed"
		if domain.HasBlockers(violations) {
			target, status = domain.StateNeedsCorrection, "Failed"
		}
		if err := volume.Transition(target); err != nil {
			return nil, err
		}
		run := domain.IntegrityCheckRun{
			ID: s.id("check"), VolumeID: volume.ID, RunNumber: len(volume.Checks) + 1, Status: status,
			CheckedVersion: volume.Version, Violations: violations, StartedAt: started, CompletedAt: s.now(),
		}
		volume.Checks = append(volume.Checks, run)
		volume.LatestCheckID = run.ID
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "integrity.completed", command.Actor, map[string]any{"checkID": run.ID, "status": status, "violations": len(violations)}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) Freeze(ctx context.Context, key, volumeID string, command VersionedCommand) (*domain.DigitizationVolume, bool, error) {
	return runCommand(ctx, s, key, "volume.freeze", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		volume, err := tx.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		if err := requireVersion(volume, command.ExpectedVersion); err != nil {
			return nil, err
		}
		if volume.State != domain.StateReadyForReview {
			return nil, domain.NewRuleError(domain.CodeForbidden, "只有通过检查的卷才能冻结")
		}
		if !hasCurrentPassedCheck(volume) {
			return nil, domain.NewRuleError(domain.CodeConflict, "完整性检查版本已过期，请重新检查后再冻结")
		}
		if err := volume.Transition(domain.StateFrozen); err != nil {
			return nil, err
		}
		digest, _ := domain.FreezeDigest(volume)
		volume.FrozenDigest = digest
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "volume.frozen", command.Actor, map[string]string{"frozenDigest": digest}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) Accession(ctx context.Context, key, volumeID string, command AccessionCommand) (*domain.DigitizationVolume, bool, error) {
	if strings.TrimSpace(command.Reviewer) == "" {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "入藏复核人不能为空")
	}
	return runCommand(ctx, s, key, "volume.accession", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		volume, err := tx.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		if err := requireVersion(volume, command.ExpectedVersion); err != nil {
			return nil, err
		}
		if volume.State != domain.StateFrozen {
			return nil, domain.NewRuleError(domain.CodeForbidden, "只有冻结卷才能签发入藏清单")
		}
		digest, pageDigests := domain.FreezeDigest(volume)
		if digest != volume.FrozenDigest {
			return nil, domain.NewRuleError(domain.CodeConflict, "冻结摘要验证失败")
		}
		now := s.now()
		manifestID := s.id("manifest")
		manifestSuffix := strings.ToUpper(domain.HashText(manifestID)[:10])
		manifest := &domain.AccessionManifest{
			ID: manifestID, VolumeID: volume.ID,
			ManifestNumber: fmt.Sprintf("AC-%s-%s", now.Format("20060102"), manifestSuffix),
			FrozenDigest:   digest, PageDigests: pageDigests, Reviewer: strings.TrimSpace(command.Reviewer),
			ReviewNote: strings.TrimSpace(command.ReviewNote), IssuedAt: now,
		}
		volume.Manifest = manifest
		if err := volume.Transition(domain.StateAccessioned); err != nil {
			return nil, err
		}
		bump(volume, now)
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "manifest.issued", command.Reviewer, map[string]string{"manifestID": manifest.ID, "manifestNumber": manifest.ManifestNumber}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}
