package selfcheck

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/httpui"
)

type Result struct {
	VolumeID        string
	ManifestID      string
	FrozenDigest    string
	AuditEventCount int
}

func Run(parent context.Context, address string, app *application.Service) (*Result, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("自检监听 %s 失败: %w", address, err)
	}
	server := &http.Server{Handler: httpui.New(app).Handler(), ReadHeaderTimeout: 3 * time.Second}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	client := &smokeClient{baseURL: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 4 * time.Second}}
	result, err := runFlow(parent, client)
	if err != nil {
		return nil, err
	}
	select {
	case serveErr := <-serveErrors:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			return nil, serveErr
		}
	default:
	}
	return result, nil
}

func runFlow(parent context.Context, client *smokeClient) (*Result, error) {
	ctx, cancel := stepContext(parent)
	var created envelope[domain.DigitizationVolume]
	_, err := client.request(ctx, http.MethodPost, "/api/volumes", "self-create", application.CreateVolume{Title: "永乐大典辑本", EditionNote: "数字校勘自检本", ShelfMark: "自检·甲一", Actor: "自检编目员"}, &created)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("建卷步骤失败: %w", err)
	}
	if created.Data.State != domain.StateDraft || created.Data.Version != 1 {
		return nil, fmt.Errorf("建卷结果状态或版本异常")
	}

	ctx, cancel = stepContext(parent)
	var replay envelope[domain.DigitizationVolume]
	response, err := client.request(ctx, http.MethodPost, "/api/volumes", "self-create", application.CreateVolume{Title: "永乐大典辑本", EditionNote: "数字校勘自检本", ShelfMark: "自检·甲一", Actor: "自检编目员"}, &replay)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("幂等重放失败: %w", err)
	}
	if !replay.Replayed || response.Header.Get("Idempotency-Replayed") != "true" || replay.Data.ID != created.Data.ID {
		return nil, fmt.Errorf("幂等请求未返回原命令结果")
	}

	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP4z8DAwMDAxAADCBYAOwAB/77J9wAAAABJRU5ErkJggg==")
	if err != nil {
		return nil, err
	}
	ctx, cancel = stepContext(parent)
	var uploaded envelope[domain.DigitizationVolume]
	err = client.upload(ctx, "/api/volumes/"+created.Data.ID+"/pages", "self-upload", map[string]string{"expectedVersion": strconv.FormatInt(created.Data.Version, 10), "folioLabel": "卷一·一叶正", "actor": "自检扫描员"}, png, &uploaded)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("上传步骤失败: %w", err)
	}
	if len(uploaded.Data.Pages) != 1 || uploaded.Data.State != domain.StateTranscribing {
		return nil, fmt.Errorf("扫描页登记结果异常")
	}
	pageID := uploaded.Data.Pages[0].ID

	ctx, cancel = stepContext(parent)
	var conflictBody map[string]any
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, client.baseURL+"/api/volumes/"+created.Data.ID, jsonBody(application.UpdateMetadata{ExpectedVersion: 1, Title: "过期修改", ShelfMark: "冲突", Actor: "自检"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "self-stale")
	conflictResponse, conflictErr := client.client.Do(req)
	if conflictErr == nil {
		defer conflictResponse.Body.Close()
		_ = decodeAny(conflictResponse, &conflictBody)
	}
	cancel()
	if conflictErr != nil || conflictResponse.StatusCode != http.StatusConflict {
		return nil, fmt.Errorf("过期版本未返回 409 Conflict")
	}

	ctx, cancel = stepContext(parent)
	var transcribed envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPut, "/api/volumes/"+created.Data.ID+"/pages/"+pageID+"/transcription", "self-transcribe", application.ReviseTranscription{ExpectedVersion: uploaded.Data.Version, Transcription: "天地玄黄，宇宙洪荒。", Actor: "自检转录员"}, &transcribed)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("转录步骤失败: %w", err)
	}

	ctx, cancel = stepContext(parent)
	var found envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPost, "/api/volumes/"+created.Data.ID+"/pages/"+pageID+"/findings", "self-find", application.AddFinding{ExpectedVersion: transcribed.Data.Version, Location: "第一行第三字", Category: domain.CategoryVariant, ObservedText: "玄", ProposedText: "玄", Actor: "自检校勘员"}, &found)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("登记疑难失败: %w", err)
	}
	findingID := found.Data.Findings[0].ID

	ctx, cancel = stepContext(parent)
	var resolved envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPost, "/api/volumes/"+created.Data.ID+"/findings/"+findingID+"/resolve", "self-resolve", application.ResolveFinding{ExpectedVersion: found.Data.Version, Resolution: "据同版复核，维持原字", ResolvedBy: "自检校勘负责人"}, &resolved)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("处理疑难失败: %w", err)
	}

	ctx, cancel = stepContext(parent)
	var checked envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPost, "/api/volumes/"+created.Data.ID+"/checks", "self-check", application.VersionedCommand{ExpectedVersion: resolved.Data.Version, Actor: "自检校勘负责人"}, &checked)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("完整性检查失败: %w", err)
	}
	if checked.Data.State != domain.StateReadyForReview {
		return nil, fmt.Errorf("完整性检查未进入待复核状态：%s", checked.Data.State)
	}

	ctx, cancel = stepContext(parent)
	var frozen envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPost, "/api/volumes/"+created.Data.ID+"/freeze", "self-freeze", application.VersionedCommand{ExpectedVersion: checked.Data.Version, Actor: "自检校勘负责人"}, &frozen)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("冻结步骤失败: %w", err)
	}
	if frozen.Data.FrozenDigest == "" {
		return nil, fmt.Errorf("冻结摘要为空")
	}

	ctx, cancel = stepContext(parent)
	var accessioned envelope[domain.DigitizationVolume]
	_, err = client.request(ctx, http.MethodPost, "/api/volumes/"+created.Data.ID+"/accession", "self-accession", application.AccessionCommand{ExpectedVersion: frozen.Data.Version, Reviewer: "自检入藏复核人", ReviewNote: "影文与清单摘要一致"}, &accessioned)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("入藏步骤失败: %w", err)
	}
	if accessioned.Data.State != domain.StateAccessioned || accessioned.Data.Manifest == nil {
		return nil, fmt.Errorf("最终状态或入藏清单异常")
	}

	ctx, cancel = stepContext(parent)
	var view envelope[application.WorkbenchView]
	_, err = client.request(ctx, http.MethodGet, "/api/volumes/"+created.Data.ID, "", nil, &view)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("查询最终工作台失败: %w", err)
	}
	if view.Data.ManifestValid == nil || !*view.Data.ManifestValid || view.Data.Volume.FrozenDigest != frozen.Data.FrozenDigest {
		return nil, fmt.Errorf("入藏清单摘要验证失败")
	}

	ctx, cancel = stepContext(parent)
	var audit envelope[[]application.AuditEvent]
	_, err = client.request(ctx, http.MethodGet, "/api/volumes/"+created.Data.ID+"/audit", "", nil, &audit)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("审计查询失败: %w", err)
	}
	if len(audit.Data) < 8 {
		return nil, fmt.Errorf("审计事件不足：%d", len(audit.Data))
	}
	return &Result{VolumeID: created.Data.ID, ManifestID: accessioned.Data.Manifest.ID, FrozenDigest: frozen.Data.FrozenDigest, AuditEventCount: len(audit.Data)}, nil
}
