package image_media_state_test

import (
	"context"
	"encoding/base64"
	"io"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
)

func TestImageResponseUsesDecodedMediaType(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := application.NewService(store)
	ctx := context.Background()
	volume, _, err := service.CreateVolume(ctx, "media-create", application.CreateVolume{Title: "图像类型测试", ShelfMark: "甲一"})
	if err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAFElEQVR4nGP4z8DAwMDAxAADCBYAOwAB/77J9wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	volume, _, err = service.RegisterPage(ctx, "media-upload", volume.ID, application.RegisterPage{
		ExpectedVersion: volume.Version,
		FolioLabel:      "一叶正",
		MediaType:       "image/jpeg",
		Data:            png,
	})
	if err != nil {
		t.Fatal(err)
	}
	reader, mediaType, _, err := service.OpenPageImage(ctx, volume.Pages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	if mediaType != "image/png" {
		t.Fatalf("PNG 图像经跨层存储后返回了错误 Content-Type: %s", mediaType)
	}
}
