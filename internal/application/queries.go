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
		return cloneWorkbenchView(cached), nil
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
		frozen := cloneWorkbenchView(view)
		s.workbenchMu.Lock()
		if existing := s.workbenchCache[volumeID]; existing != nil {
			view = cloneWorkbenchView(existing)
		} else {
			s.workbenchCache[volumeID] = frozen
			view = cloneWorkbenchView(frozen)
		}
		s.workbenchMu.Unlock()
	}
	return view, nil
}

func cloneWorkbenchView(src *WorkbenchView) *WorkbenchView {
	if src == nil {
		return nil
	}
	dst := WorkbenchView{ManifestDigest: src.ManifestDigest}
	if src.ManifestValid != nil {
		manifestValid := *src.ManifestValid
		dst.ManifestValid = &manifestValid
	}
	if src.Volume != nil {
		volume := *src.Volume
		volume.PageOrder = append([]string(nil), src.Volume.PageOrder...)
		if src.Volume.Pages != nil {
			volume.Pages = make([]domain.FacsimilePage, len(src.Volume.Pages))
			copy(volume.Pages, src.Volume.Pages)
		}
		if src.Volume.Findings != nil {
			volume.Findings = make([]domain.CollationFinding, len(src.Volume.Findings))
			copy(volume.Findings, src.Volume.Findings)
			for index := range volume.Findings {
				if src.Volume.Findings[index].ResolvedAt != nil {
					resolvedAt := *src.Volume.Findings[index].ResolvedAt
					volume.Findings[index].ResolvedAt = &resolvedAt
				}
			}
		}
		if src.Volume.Checks != nil {
			volume.Checks = make([]domain.IntegrityCheckRun, len(src.Volume.Checks))
			copy(volume.Checks, src.Volume.Checks)
			for index := range volume.Checks {
				if src.Volume.Checks[index].Violations != nil {
					volume.Checks[index].Violations = make([]domain.IntegrityViolation, len(src.Volume.Checks[index].Violations))
					copy(volume.Checks[index].Violations, src.Volume.Checks[index].Violations)
				}
			}
		}
		if src.Volume.Manifest != nil {
			manifest := *src.Volume.Manifest
			if src.Volume.Manifest.PageDigests != nil {
				manifest.PageDigests = make([]domain.PageDigest, len(src.Volume.Manifest.PageDigests))
				copy(manifest.PageDigests, src.Volume.Manifest.PageDigests)
			}
			volume.Manifest = &manifest
		}
		dst.Volume = &volume
	}
	if src.OrderedPages != nil {
		dst.OrderedPages = make([]domain.FacsimilePage, len(src.OrderedPages))
		copy(dst.OrderedPages, src.OrderedPages)
	}
	if src.OpenFindings != nil {
		dst.OpenFindings = make([]domain.CollationFinding, len(src.OpenFindings))
		copy(dst.OpenFindings, src.OpenFindings)
		for index := range dst.OpenFindings {
			if src.OpenFindings[index].ResolvedAt != nil {
				resolvedAt := *src.OpenFindings[index].ResolvedAt
				dst.OpenFindings[index].ResolvedAt = &resolvedAt
			}
		}
	}
	if src.LatestCheck != nil {
		latest := *src.LatestCheck
		if src.LatestCheck.Violations != nil {
			latest.Violations = make([]domain.IntegrityViolation, len(src.LatestCheck.Violations))
			copy(latest.Violations, src.LatestCheck.Violations)
		}
		dst.LatestCheck = &latest
	}
	return &dst
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
