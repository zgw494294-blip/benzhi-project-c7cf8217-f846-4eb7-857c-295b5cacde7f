package application

import (
	"context"
	"strings"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type AddFinding struct {
	ExpectedVersion int64                  `json:"expectedVersion"`
	Location        string                 `json:"location"`
	Category        domain.FindingCategory `json:"category"`
	ObservedText    string                 `json:"observedText"`
	ProposedText    string                 `json:"proposedText"`
	Actor           string                 `json:"actor"`
}

type ResolveFinding struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Resolution      string `json:"resolution"`
	ResolvedBy      string `json:"resolvedBy"`
}

func (s *Service) AddFinding(ctx context.Context, key, volumeID, pageID string, command AddFinding) (*domain.DigitizationVolume, bool, error) {
	if err := domain.ValidateFinding(command.Category, command.Location); err != nil {
		return nil, false, err
	}
	return runCommand(ctx, s, key, "finding.add", volumeID+"/"+pageID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		volume, err := tx.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		if err := requireVersion(volume, command.ExpectedVersion); err != nil {
			return nil, err
		}
		if err := volume.EnsureEditable(); err != nil {
			return nil, err
		}
		if _, err := pageByID(volume, pageID); err != nil {
			return nil, err
		}
		finding := domain.CollationFinding{
			ID: s.id("finding"), PageID: pageID, Location: strings.TrimSpace(command.Location), Category: command.Category,
			ObservedText: command.ObservedText, ProposedText: command.ProposedText, Status: domain.FindingOpen,
		}
		volume.Findings = append(volume.Findings, finding)
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(tx, volume, "finding.added", command.Actor, finding); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) ResolveFinding(ctx context.Context, key, volumeID, findingID string, command ResolveFinding) (*domain.DigitizationVolume, bool, error) {
	if strings.TrimSpace(command.Resolution) == "" || strings.TrimSpace(command.ResolvedBy) == "" {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "处理依据和处理人不能为空")
	}
	return runCommand(ctx, s, key, "finding.resolve", volumeID+"/"+findingID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		volume, err := tx.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		if err := requireVersion(volume, command.ExpectedVersion); err != nil {
			return nil, err
		}
		if err := volume.EnsureEditable(); err != nil {
			return nil, err
		}
		finding, err := findingByID(volume, findingID)
		if err != nil {
			return nil, err
		}
		if finding.Status == domain.FindingResolved {
			return nil, domain.NewRuleError(domain.CodeConflict, "校勘问题已经处理")
		}
		now := s.now()
		finding.Status = domain.FindingResolved
		finding.Resolution = strings.TrimSpace(command.Resolution)
		finding.ResolvedBy = strings.TrimSpace(command.ResolvedBy)
		finding.ResolvedAt = &now
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, now)
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(tx, volume, "finding.resolved", command.ResolvedBy, map[string]string{"findingID": finding.ID, "resolution": finding.Resolution}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}
