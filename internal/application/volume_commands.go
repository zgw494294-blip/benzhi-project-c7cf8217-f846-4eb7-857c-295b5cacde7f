package application

import (
	"context"
	"strings"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type CreateVolume struct {
	Title       string `json:"title"`
	EditionNote string `json:"editionNote"`
	ShelfMark   string `json:"shelfMark"`
	Actor       string `json:"actor"`
}

type UpdateMetadata struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Title           string `json:"title"`
	EditionNote     string `json:"editionNote"`
	ShelfMark       string `json:"shelfMark"`
	Actor           string `json:"actor"`
}

func (s *Service) CreateVolume(ctx context.Context, key string, command CreateVolume) (*domain.DigitizationVolume, bool, error) {
	if err := domain.ValidateMetadata(command.Title, command.ShelfMark); err != nil {
		return nil, false, err
	}
	return runCommand(ctx, s, key, "volume.create", "", command, func(tx Transaction) (*domain.DigitizationVolume, error) {
		now := s.now()
		volume := &domain.DigitizationVolume{
			ID: s.id("vol"), Title: strings.TrimSpace(command.Title),
			EditionNote: strings.TrimSpace(command.EditionNote), ShelfMark: strings.TrimSpace(command.ShelfMark),
			State: domain.StateDraft, Version: 1, PageOrder: []string{}, Pages: []domain.FacsimilePage{},
			Findings: []domain.CollationFinding{}, Checks: []domain.IntegrityCheckRun{}, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "volume.created", command.Actor, map[string]string{"title": volume.Title}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) UpdateMetadata(ctx context.Context, key, volumeID string, command UpdateMetadata) (*domain.DigitizationVolume, bool, error) {
	if err := domain.ValidateMetadata(command.Title, command.ShelfMark); err != nil {
		return nil, false, err
	}
	return runCommand(ctx, s, key, "volume.metadata", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
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
		volume.Title = strings.TrimSpace(command.Title)
		volume.EditionNote = strings.TrimSpace(command.EditionNote)
		volume.ShelfMark = strings.TrimSpace(command.ShelfMark)
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "volume.metadata_updated", command.Actor, map[string]string{"title": volume.Title}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}
