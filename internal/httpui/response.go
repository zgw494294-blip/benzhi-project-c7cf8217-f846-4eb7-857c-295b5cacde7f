package httpui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

const maxJSONBody = 1 << 20

type responseEnvelope struct {
	Data     any  `json:"data"`
	Replayed bool `json:"replayed,omitempty"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeResult(w http.ResponseWriter, status int, data any, replayed bool) {
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, status, responseEnvelope{Data: data, Replayed: replayed})
}

func writeError(w http.ResponseWriter, err error) {
	code := application.ErrorCode(err)
	status := http.StatusInternalServerError
	switch code {
	case domain.CodeInvalid:
		status = http.StatusBadRequest
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeConflict:
		status = http.StatusConflict
	case domain.CodeForbidden:
		status = http.StatusUnprocessableEntity
	}
	message := "服务器处理请求失败"
	if code != "internal" {
		message = err.Error()
	}
	var envelope errorEnvelope
	envelope.Error.Code = string(code)
	envelope.Error.Message = message
	writeJSON(w, status, envelope)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.NewRuleError(domain.CodeInvalid, "请求体不能为空")
		}
		return domain.NewRuleError(domain.CodeInvalid, "JSON 请求无效：%v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.NewRuleError(domain.CodeInvalid, "JSON 请求只能包含一个对象")
	}
	return nil
}

func requireIdempotency(r *http.Request) (string, error) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		return "", domain.NewRuleError(domain.CodeInvalid, "缺少 Idempotency-Key 请求头")
	}
	if len(key) > 160 {
		return "", domain.NewRuleError(domain.CodeInvalid, "Idempotency-Key 过长")
	}
	return key, nil
}

func pathID(r *http.Request, name string) (string, error) {
	value := r.PathValue(name)
	if value == "" {
		return "", fmt.Errorf("路径参数 %s 缺失", name)
	}
	return value, nil
}
