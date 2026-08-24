package application

import (
	"context"
	"io"
	"sort"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type VolumeSummary struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	ShelfMark    string             `json:"shelfMark"`
	State        domain.VolumeState `json:"state"`
	Version      int64              `json:"version"`
	PageCount    int                `json:"pageCount"`
	OpenFindings int                `json:"openFindings"`
	UpdatedAt    string             `json:"updatedAt"`
}

type WorkbenchView struct {
	Volume         *domain.DigitizationVolume `json:"volume"`
	OrderedPages   []domain.FacsimilePage     `json:"orderedPages"`
	OpenFindings   []domain.CollationFinding  `json:"openFindings"`
	LatestCheck    *domain.IntegrityCheckRun  `json:"latestCheck,omitempty"`
	ManifestValid  *bool                      `json:"manifestValid,omitempty"`
	ManifestDigest string                     `json:"manifestDigest,omitempty"`
}

func (s *Service) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	volumes, err := s.store.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]VolumeSummary, 0, len(volumes))
	for _, volume := range volumes {
		open := 0
		for _, finding := range volume.Findings {
			if finding.Status == domain.FindingOpen {
				open++
			}
		}
		result = append(result, VolumeSummary{ID: volume.ID, Title: volume.Title, ShelfMark: volume.ShelfMark, State: volume.State, Version: volume.Version, PageCount: len(volume.Pages), OpenFindings: open, UpdatedAt: volume.UpdatedAt.Format("2006-01-02 15:04")})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt > result[j].UpdatedAt })
	return result, nil
}

func (s *Service) GetWorkbench(ctx context.Context, volumeID string) (*WorkbenchView, error) {
	s.workbenchMu.RLock()
	cached := s.workbenchCache[volumeID]
	s.workbenchMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	volume, err := s.store.GetVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	view := &WorkbenchView{Volume: volume, OrderedPages: domain.OrderedPages(volume), OpenFindings: []domain.CollationFinding{}}
	for _, finding := range volume.Findings {
		if finding.Status == domain.FindingOpen {
			view.OpenFindings = append(view.OpenFindings, finding)
		}
	}
	if len(volume.Checks) > 0 {
		view.LatestCheck = &volume.Checks[len(volume.Checks)-1]
	}
	if volume.Manifest != nil {
		valid := domain.VerifyAccessionManifest(volume)
		view.ManifestValid = &valid
		view.ManifestDigest = domain.ManifestDigest(*volume.Manifest)
	}
	if volume.State == domain.StateAccessioned {
		s.workbenchMu.Lock()
		if cached := s.workbenchCache[volumeID]; cached != nil {
			view = cached
		} else {
			s.workbenchCache[volumeID] = view
		}
		s.workbenchMu.Unlock()
	}
	return view, nil
}

func (s *Service) ListAudit(ctx context.Context, volumeID string) ([]AuditEvent, error) {
	return s.store.ListAudit(ctx, volumeID)
}

func (s *Service) OpenPageImage(ctx context.Context, pageID string) (io.ReadCloser, string, int64, error) {
	volumes, err := s.store.ListVolumes(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	for _, volume := range volumes {
		for _, page := range volume.Pages {
			if page.ID == pageID {
				return s.store.OpenImage(ctx, page.ImageObjectKey)
			}
		}
	}
	return nil, "", 0, domain.NewRuleError(domain.CodeNotFound, "页面图像不存在")
}
