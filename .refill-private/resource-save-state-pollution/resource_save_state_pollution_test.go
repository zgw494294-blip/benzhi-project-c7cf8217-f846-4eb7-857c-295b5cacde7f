package resource_save_state_pollution_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"path/filepath"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
)

func TestEachOpenPageImageHasIndependentReadState(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "collation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	ctx := context.Background()

	volume, _, err := service.CreateVolume(ctx, "create-volume", application.CreateVolume{
		Title: "图像资源复用测试本", ShelfMark: "测试·资源", Actor: "测试员",
	})
	if err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP4z8DAwMDAxAADCBYAOwAB/77J9wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RegisterPage(ctx, "register-page", volume.ID, application.RegisterPage{
		ExpectedVersion: volume.Version, FolioLabel: "一叶正", MediaType: "image/png", Data: png, Actor: "扫描员",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, _, _, err := service.OpenPageImage(ctx, volume.Pages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	prefix := make([]byte, 8)
	if _, err := io.ReadFull(first, prefix); err != nil {
		t.Fatal(err)
	}

	second, _, size, err := service.OpenPageImage(ctx, volume.Pages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	got, err := io.ReadAll(second)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(png)) || !bytes.Equal(got, png) {
		t.Fatalf("第二次打开复用了第一次读取游标: declared=%d got=%d want=%d", size, len(got), len(png))
	}
}
