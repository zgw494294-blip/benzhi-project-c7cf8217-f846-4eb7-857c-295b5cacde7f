package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

type repository struct{ tx *sql.Tx }

func (r *repository) GetVolume(ctx context.Context, id string) (*domain.DigitizationVolume, error) {
	return loadVolume(ctx, r.tx, id)
}

func (r *repository) SaveImage(ctx context.Context, image application.ImageObject) error {
	if image.Key == "" || image.SHA256 != image.Key || domain.HashBytes(image.Data) != image.Key {
		return domain.NewRuleError(domain.CodeInvalid, "图像对象摘要无效")
	}
	var existing []byte
	err := r.tx.QueryRowContext(ctx, `SELECT data FROM image_objects WHERE object_key = ?`, image.Key).Scan(&existing)
	if err == nil {
		if domain.HashBytes(existing) != image.Key {
			return domain.NewRuleError(domain.CodeConflict, "已有图像对象内容与摘要不符")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = r.tx.ExecContext(ctx, `INSERT INTO image_objects(object_key,media_type,sha256,byte_size,data) VALUES(?,?,?,?,?)`, image.Key, image.MediaType, image.SHA256, len(image.Data), image.Data)
	return err
}

func (r *repository) AppendAudit(ctx context.Context, event application.AuditEvent) error {
	_, err := r.tx.ExecContext(ctx, `INSERT INTO audit_events(id,volume_id,operation,actor,version,occurred_at,details_json) VALUES(?,?,?,?,?,?,?)`, event.ID, event.VolumeID, event.Operation, event.Actor, event.Version, event.OccurredAt.Format(time.RFC3339Nano), []byte(event.Details))
	return err
}

func (r *repository) SaveVolume(ctx context.Context, volume *domain.DigitizationVolume) error {
	orderJSON, err := json.Marshal(volume.PageOrder)
	if err != nil {
		return err
	}
	_, err = r.tx.ExecContext(ctx, `INSERT INTO volumes(id,title,edition_note,shelf_mark,state,version,page_order_json,latest_check_id,frozen_digest,created_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,edition_note=excluded.edition_note,shelf_mark=excluded.shelf_mark,state=excluded.state,version=excluded.version,page_order_json=excluded.page_order_json,latest_check_id=excluded.latest_check_id,frozen_digest=excluded.frozen_digest,updated_at=excluded.updated_at`,
		volume.ID, volume.Title, volume.EditionNote, volume.ShelfMark, volume.State, volume.Version, orderJSON,
		volume.LatestCheckID, volume.FrozenDigest, volume.CreatedAt.Format(time.RFC3339Nano), volume.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	for _, table := range []string{"pages", "findings", "check_runs", "manifests"} {
		if _, err := r.tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE volume_id = ?`, volume.ID); err != nil {
			return err
		}
	}
	for _, page := range volume.Pages {
		_, err = r.tx.ExecContext(ctx, `INSERT INTO pages(id,volume_id,folio_label,sequence,image_object_key,media_type,byte_size,sha256,width,height,transcription,revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			page.ID, volume.ID, page.FolioLabel, page.Sequence, page.ImageObjectKey, page.MediaType, page.ByteSize, page.SHA256, page.Width, page.Height, page.Transcription, page.Revision)
		if err != nil {
			return err
		}
	}
	for _, finding := range volume.Findings {
		var resolvedAt any
		if finding.ResolvedAt != nil {
			resolvedAt = finding.ResolvedAt.Format(time.RFC3339Nano)
		}
		_, err = r.tx.ExecContext(ctx, `INSERT INTO findings(id,volume_id,page_id,location,category,observed_text,proposed_text,status,resolution,resolved_by,resolved_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			finding.ID, volume.ID, finding.PageID, finding.Location, finding.Category, finding.ObservedText, finding.ProposedText, finding.Status, finding.Resolution, finding.ResolvedBy, resolvedAt)
		if err != nil {
			return err
		}
	}
	for _, run := range volume.Checks {
		violations, _ := json.Marshal(run.Violations)
		_, err = r.tx.ExecContext(ctx, `INSERT INTO check_runs(id,volume_id,run_number,status,checked_version,violations_json,started_at,completed_at) VALUES(?,?,?,?,?,?,?,?)`,
			run.ID, volume.ID, run.RunNumber, run.Status, run.CheckedVersion, violations, run.StartedAt.Format(time.RFC3339Nano), run.CompletedAt.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	if volume.Manifest != nil {
		pageDigests, _ := json.Marshal(volume.Manifest.PageDigests)
		_, err = r.tx.ExecContext(ctx, `INSERT INTO manifests(id,volume_id,manifest_number,frozen_digest,page_digests_json,reviewer,review_note,issued_at) VALUES(?,?,?,?,?,?,?,?)`,
			volume.Manifest.ID, volume.ID, volume.Manifest.ManifestNumber, volume.Manifest.FrozenDigest, pageDigests, volume.Manifest.Reviewer, volume.Manifest.ReviewNote, volume.Manifest.IssuedAt.Format(time.RFC3339Nano))
	}
	return err
}

func loadVolume(ctx context.Context, q queryer, id string) (*domain.DigitizationVolume, error) {
	volume := &domain.DigitizationVolume{}
	var state, created, updated string
	var pageOrder []byte
	err := q.QueryRowContext(ctx, `SELECT id,title,edition_note,shelf_mark,state,version,page_order_json,latest_check_id,frozen_digest,created_at,updated_at FROM volumes WHERE id=?`, id).Scan(
		&volume.ID, &volume.Title, &volume.EditionNote, &volume.ShelfMark, &state, &volume.Version, &pageOrder, &volume.LatestCheckID, &volume.FrozenDigest, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewRuleError(domain.CodeNotFound, "数字化卷不存在")
	}
	if err != nil {
		return nil, err
	}
	volume.State = domain.VolumeState(state)
	volume.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	volume.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if err := json.Unmarshal(pageOrder, &volume.PageOrder); err != nil {
		return nil, fmt.Errorf("解析页序: %w", err)
	}
	if err := loadPages(ctx, q, volume); err != nil {
		return nil, err
	}
	if err := loadFindings(ctx, q, volume); err != nil {
		return nil, err
	}
	if err := loadChecks(ctx, q, volume); err != nil {
		return nil, err
	}
	if err := loadManifest(ctx, q, volume); err != nil {
		return nil, err
	}
	return volume, nil
}

func loadPages(ctx context.Context, q queryer, volume *domain.DigitizationVolume) error {
	rows, err := q.QueryContext(ctx, `SELECT id,volume_id,folio_label,sequence,image_object_key,media_type,byte_size,sha256,width,height,transcription,revision FROM pages WHERE volume_id=? ORDER BY sequence,id`, volume.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	volume.Pages = []domain.FacsimilePage{}
	for rows.Next() {
		var p domain.FacsimilePage
		if err := rows.Scan(&p.ID, &p.VolumeID, &p.FolioLabel, &p.Sequence, &p.ImageObjectKey, &p.MediaType, &p.ByteSize, &p.SHA256, &p.Width, &p.Height, &p.Transcription, &p.Revision); err != nil {
			return err
		}
		volume.Pages = append(volume.Pages, p)
	}
	return rows.Err()
}

func loadFindings(ctx context.Context, q queryer, volume *domain.DigitizationVolume) error {
	rows, err := q.QueryContext(ctx, `SELECT id,page_id,location,category,observed_text,proposed_text,status,resolution,resolved_by,resolved_at FROM findings WHERE volume_id=? ORDER BY id`, volume.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	volume.Findings = []domain.CollationFinding{}
	for rows.Next() {
		var f domain.CollationFinding
		var category, status string
		var resolved sql.NullString
		if err := rows.Scan(&f.ID, &f.PageID, &f.Location, &category, &f.ObservedText, &f.ProposedText, &status, &f.Resolution, &f.ResolvedBy, &resolved); err != nil {
			return err
		}
		f.Category = domain.FindingCategory(category)
		f.Status = domain.FindingStatus(status)
		if resolved.Valid {
			t, _ := time.Parse(time.RFC3339Nano, resolved.String)
			f.ResolvedAt = &t
		}
		volume.Findings = append(volume.Findings, f)
	}
	return rows.Err()
}

func loadChecks(ctx context.Context, q queryer, volume *domain.DigitizationVolume) error {
	rows, err := q.QueryContext(ctx, `SELECT id,run_number,status,checked_version,violations_json,started_at,completed_at FROM check_runs WHERE volume_id=? ORDER BY run_number`, volume.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	volume.Checks = []domain.IntegrityCheckRun{}
	for rows.Next() {
		var run domain.IntegrityCheckRun
		var raw []byte
		var started, completed string
		run.VolumeID = volume.ID
		if err := rows.Scan(&run.ID, &run.RunNumber, &run.Status, &run.CheckedVersion, &raw, &started, &completed); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &run.Violations); err != nil {
			return err
		}
		run.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		run.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		volume.Checks = append(volume.Checks, run)
	}
	return rows.Err()
}

func loadManifest(ctx context.Context, q queryer, volume *domain.DigitizationVolume) error {
	var manifest domain.AccessionManifest
	var raw []byte
	var issued string
	err := q.QueryRowContext(ctx, `SELECT id,manifest_number,frozen_digest,page_digests_json,reviewer,review_note,issued_at FROM manifests WHERE volume_id=?`, volume.ID).Scan(&manifest.ID, &manifest.ManifestNumber, &manifest.FrozenDigest, &raw, &manifest.Reviewer, &manifest.ReviewNote, &issued)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	manifest.VolumeID = volume.ID
	manifest.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued)
	if err := json.Unmarshal(raw, &manifest.PageDigests); err != nil {
		return err
	}
	volume.Manifest = &manifest
	return nil
}
