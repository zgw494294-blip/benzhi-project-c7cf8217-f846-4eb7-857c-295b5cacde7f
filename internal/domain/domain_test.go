package domain

import "testing"

func TestIntegrityAndDigestAreDeterministic(t *testing.T) {
	volume := &DigitizationVolume{ID: "v1", Title: "测试古籍", ShelfMark: "甲一", PageOrder: []string{"p1"}, Pages: []FacsimilePage{{ID: "p1", FolioLabel: "一叶正", Sequence: 1, ImageObjectKey: HashText("image"), SHA256: HashText("image"), Width: 2, Height: 2, Transcription: "天地", Revision: 1}}}
	if violations := CheckIntegrity(volume); len(violations) != 0 {
		t.Fatalf("预期无违规，得到 %#v", violations)
	}
	first, firstPages := FreezeDigest(volume)
	second, secondPages := FreezeDigest(volume)
	if first == "" || first != second || firstPages[0].Combined != secondPages[0].Combined {
		t.Fatal("冻结摘要不确定")
	}
	volume.Pages[0].Transcription = "天地玄黄"
	changed, _ := FreezeDigest(volume)
	if changed == first {
		t.Fatal("转录变化后摘要未变化")
	}
}

func TestStateTransitionsRejectFrozenEditing(t *testing.T) {
	volume := &DigitizationVolume{State: StateReadyForReview}
	if err := volume.Transition(StateFrozen); err != nil {
		t.Fatal(err)
	}
	if err := volume.EnsureEditable(); err == nil {
		t.Fatal("冻结卷仍允许编辑")
	}
	if err := volume.Transition(StateTranscribing); err == nil {
		t.Fatal("非法迁移未被拒绝")
	}
}

func TestManifestVerificationIncludesEveryOrderedPage(t *testing.T) {
	volume := &DigitizationVolume{ID: "v1", Title: "测试古籍", ShelfMark: "甲一", PageOrder: []string{"p1"}, Pages: []FacsimilePage{{ID: "p1", FolioLabel: "一叶正", Sequence: 1, ImageObjectKey: HashText("image"), SHA256: HashText("image"), Transcription: "天地", Revision: 1}}}
	digest, pages := FreezeDigest(volume)
	volume.FrozenDigest = digest
	volume.Manifest = &AccessionManifest{ID: "m1", VolumeID: volume.ID, FrozenDigest: digest, PageDigests: pages}
	if !VerifyAccessionManifest(volume) {
		t.Fatal("有效清单未通过验证")
	}
	volume.Manifest.PageDigests = nil
	if VerifyAccessionManifest(volume) {
		t.Fatal("缺少逐页摘要的清单仍通过验证")
	}
}
