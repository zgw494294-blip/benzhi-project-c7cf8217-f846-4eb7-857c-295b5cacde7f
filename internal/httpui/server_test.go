package httpui_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/httpui"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/storage"
)

func TestIndexAndIdempotentCreateContract(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := httptest.NewServer(httpui.New(application.NewService(store)).Handler())
	defer server.Close()

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	index, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(index), "古籍数字校勘入藏台") || response.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("工作台首页或安全响应头不完整")
	}

	command := application.CreateVolume{Title: "接口测试本", ShelfMark: "测·一", Actor: "测试员"}
	first := postVolume(t, server.URL, "same-key", command)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("首次建卷返回 %d", first.StatusCode)
	}
	first.Body.Close()
	replay := postVolume(t, server.URL, "same-key", command)
	if replay.StatusCode != http.StatusOK || replay.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatalf("相同请求未幂等重放: %d", replay.StatusCode)
	}
	replay.Body.Close()
	conflict := postVolume(t, server.URL, "same-key", application.CreateVolume{Title: "不同载荷", ShelfMark: "测·二", Actor: "测试员"})
	if conflict.StatusCode != http.StatusConflict {
		t.Fatalf("同一幂等键的不同载荷应冲突，得到 %d", conflict.StatusCode)
	}
	conflict.Body.Close()
}

func postVolume(t *testing.T, baseURL, key string, command application.CreateVolume) *http.Response {
	t.Helper()
	raw, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/volumes", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
