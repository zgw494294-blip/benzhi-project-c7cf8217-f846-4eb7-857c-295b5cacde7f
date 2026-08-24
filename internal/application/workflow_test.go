package application_test

import (
	"context"
	"encoding/base64"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
)

func TestCorrectionFlowCanFixFolioAndAccession(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	ctx := context.Background()

	volume, _, err := service.CreateVolume(ctx, "create", application.CreateVolume{Title: "校勘测试本", ShelfMark: "善本·甲一", Actor: "编目员"})
	if err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP4z8DAwMDAxAADCBYAOwAB/77J9wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RegisterPage(ctx, "upload", volume.ID, application.RegisterPage{ExpectedVersion: volume.Version, MediaType: "image/png", Data: png, Actor: "扫描员"})
	if err != nil {
		t.Fatal(err)
	}
	pageID := volume.Pages[0].ID
	volume, _, err = service.ReviseTranscription(ctx, "transcribe", volume.ID, pageID, application.ReviseTranscription{ExpectedVersion: volume.Version, Transcription: "天地玄黄", Actor: "转录员"})
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RunIntegrityCheck(ctx, "check-failed", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "校勘员"})
	if err != nil || volume.State != domain.StateNeedsCorrection {
		t.Fatalf("缺少叶号时应进入待整改状态: %v, %s", err, volume.State)
	}
	volume, _, err = service.UpdatePageMetadata(ctx, "fix-folio", volume.ID, pageID, application.UpdatePageMetadata{ExpectedVersion: volume.Version, FolioLabel: "卷一·一叶正", Actor: "校勘员"})
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RunIntegrityCheck(ctx, "check-passed", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "负责人"})
	if err != nil || volume.State != domain.StateReadyForReview {
		t.Fatalf("整改后复检未通过: %v, %s", err, volume.State)
	}
	volume, _, err = service.Freeze(ctx, "freeze", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "负责人"})
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.Accession(ctx, "accession", volume.ID, application.AccessionCommand{ExpectedVersion: volume.Version, Reviewer: "入藏复核人", ReviewNote: "核对无误"})
	if err != nil || volume.State != domain.StateAccessioned || !domain.VerifyAccessionManifest(volume) {
		t.Fatalf("入藏清单异常: %v", err)
	}
	events, err := store.ListAudit(ctx, volume.ID)
	if err != nil || len(events) != 8 || events[4].Operation != "page.metadata_updated" {
		t.Fatalf("审计轨迹未完整记录整改流程: %v, %#v", err, events)
	}
}

func TestEditAfterPassedCheckRequiresAnotherCheck(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	ctx := context.Background()
	volume, _, err := service.CreateVolume(ctx, "recheck-create", application.CreateVolume{Title: "复检测试本", ShelfMark: "善本·乙二", Actor: "编目员"})
	if err != nil {
		t.Fatal(err)
	}
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP4z8DAwMDAxAADCBYAOwAB/77J9wAAAABJRU5ErkJggg==")
	volume, _, err = service.RegisterPage(ctx, "recheck-upload", volume.ID, application.RegisterPage{ExpectedVersion: volume.Version, FolioLabel: "一叶正", MediaType: "image/png", Data: png, Actor: "扫描员"})
	if err != nil {
		t.Fatal(err)
	}
	pageID := volume.Pages[0].ID
	volume, _, err = service.ReviseTranscription(ctx, "recheck-first-text", volume.ID, pageID, application.ReviseTranscription{ExpectedVersion: volume.Version, Transcription: "天地", Actor: "转录员"})
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RunIntegrityCheck(ctx, "recheck-first-check", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "校勘员"})
	if err != nil || volume.State != domain.StateReadyForReview {
		t.Fatalf("首次检查未通过: %v", err)
	}
	volume, _, err = service.ReviseTranscription(ctx, "recheck-second-text", volume.ID, pageID, application.ReviseTranscription{ExpectedVersion: volume.Version, Transcription: "天地玄黄", Actor: "转录员"})
	if err != nil || volume.State != domain.StateTranscribing {
		t.Fatalf("检查后编辑应退回转录状态: %v, %s", err, volume.State)
	}
	if _, _, err := service.Freeze(ctx, "recheck-stale-freeze", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "负责人"}); application.ErrorCode(err) != domain.CodeForbidden {
		t.Fatalf("过期检查后的冻结应被拒绝: %v", err)
	}
	volume, _, err = service.RunIntegrityCheck(ctx, "recheck-second-check", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "校勘员"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Freeze(ctx, "recheck-current-freeze", volume.ID, application.VersionedCommand{ExpectedVersion: volume.Version, Actor: "负责人"}); err != nil {
		t.Fatalf("复检后冻结失败: %v", err)
	}
}
