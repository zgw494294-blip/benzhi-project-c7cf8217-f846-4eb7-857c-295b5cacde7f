package application

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

const MaxImageBytes int64 = 24 << 20

type RegisterPage struct {
	ExpectedVersion int64
	FolioLabel      string
	MediaType       string
	Data            []byte
	ClaimedSHA256   string
	Actor           string
}

type ReorderPages struct {
	ExpectedVersion int64    `json:"expectedVersion"`
	PageOrder       []string `json:"pageOrder"`
	Actor           string   `json:"actor"`
}

type ReviseTranscription struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Transcription   string `json:"transcription"`
	Actor           string `json:"actor"`
}

type UpdatePageMetadata struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	FolioLabel      string `json:"folioLabel"`
	Actor           string `json:"actor"`
}

func (s *Service) RegisterPage(ctx context.Context, key, volumeID string, command RegisterPage) (*domain.DigitizationVolume, bool, error) {
	if len(command.Data) == 0 || int64(len(command.Data)) > MaxImageBytes {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "扫描图像为空或超过 24 MiB")
	}
	media := strings.ToLower(strings.TrimSpace(command.MediaType))
	if media != "image/jpeg" && media != "image/png" && media != "image/gif" {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "仅支持 JPEG、PNG 或 GIF 图像")
	}
	sha := domain.HashBytes(command.Data)
	if command.ClaimedSHA256 != "" && !strings.EqualFold(command.ClaimedSHA256, sha) {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "图像摘要与声明值不符")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(command.Data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 30000 || config.Height > 30000 {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "图像格式无效或尺寸越界")
	}
	return runCommand(ctx, s, key, "page.register", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
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
		pageID := s.id("page")
		page := domain.FacsimilePage{
			ID: pageID, VolumeID: volume.ID, FolioLabel: strings.TrimSpace(command.FolioLabel), Sequence: len(volume.Pages) + 1,
			ImageObjectKey: sha, MediaType: media, ByteSize: int64(len(command.Data)), SHA256: sha,
			Width: config.Width, Height: config.Height, Revision: 0,
		}
		if err := tx.SaveImage(ctx, ImageObject{Key: sha, MediaType: media, Data: command.Data, SHA256: sha}); err != nil {
			return nil, err
		}
		volume.Pages = append(volume.Pages, page)
		volume.PageOrder = append(volume.PageOrder, page.ID)
		if volume.State == domain.StateDraft {
			if err := volume.Transition(domain.StateTranscribing); err != nil {
				return nil, err
			}
		} else if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "page.registered", command.Actor, map[string]any{"pageID": page.ID, "folioLabel": page.FolioLabel, "sha256": sha}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) ReorderPages(ctx context.Context, key, volumeID string, command ReorderPages) (*domain.DigitizationVolume, bool, error) {
	return runCommand(ctx, s, key, "pages.reorder", volumeID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
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
		if err := domain.ValidatePageOrder(volume, command.PageOrder); err != nil {
			return nil, err
		}
		volume.PageOrder = append([]string(nil), command.PageOrder...)
		byID := make(map[string]*domain.FacsimilePage, len(volume.Pages))
		for index := range volume.Pages {
			byID[volume.Pages[index].ID] = &volume.Pages[index]
		}
		for index, id := range volume.PageOrder {
			byID[id].Sequence = index + 1
		}
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "pages.reordered", command.Actor, map[string]any{"pageOrder": volume.PageOrder}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) UpdatePageMetadata(ctx context.Context, key, volumeID, pageID string, command UpdatePageMetadata) (*domain.DigitizationVolume, bool, error) {
	if len([]rune(command.FolioLabel)) > 100 {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "叶号超过允许长度")
	}
	return runCommand(ctx, s, key, "page.metadata", volumeID+"/"+pageID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
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
		page, err := pageByID(volume, pageID)
		if err != nil {
			return nil, err
		}
		previous := page.FolioLabel
		page.FolioLabel = strings.TrimSpace(command.FolioLabel)
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "page.metadata_updated", command.Actor, map[string]string{"pageID": page.ID, "previousFolioLabel": previous, "folioLabel": page.FolioLabel}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}

func (s *Service) ReviseTranscription(ctx context.Context, key, volumeID, pageID string, command ReviseTranscription) (*domain.DigitizationVolume, bool, error) {
	if len([]rune(command.Transcription)) > 200000 {
		return nil, false, domain.NewRuleError(domain.CodeInvalid, "单页转录超过允许长度")
	}
	return runCommand(ctx, s, key, "page.transcription", volumeID+"/"+pageID, command, func(tx Transaction) (*domain.DigitizationVolume, error) {
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
		page, err := pageByID(volume, pageID)
		if err != nil {
			return nil, err
		}
		page.Transcription = strings.ReplaceAll(command.Transcription, "\r\n", "\n")
		page.Revision++
		if err := markEdited(volume); err != nil {
			return nil, err
		}
		bump(volume, s.now())
		if err := tx.SaveVolume(ctx, volume); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, volume, "page.transcription_revised", command.Actor, map[string]any{"pageID": page.ID, "revision": page.Revision}); err != nil {
			return nil, err
		}
		return volume, nil
	})
}
