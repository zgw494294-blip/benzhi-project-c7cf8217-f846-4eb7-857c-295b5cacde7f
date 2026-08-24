package httpui

import (
	"io"
	"net/http"
	"strconv"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/domain"
)

func (s *Server) HandleRegisterPage(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, application.MaxImageBytes+(1<<20))
	if err := r.ParseMultipartForm(application.MaxImageBytes); err != nil {
		writeError(w, domain.NewRuleError(domain.CodeInvalid, "上传表单无效：%v", err))
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, domain.NewRuleError(domain.CodeInvalid, "缺少 image 文件字段"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, application.MaxImageBytes+1))
	if err != nil {
		writeError(w, domain.NewRuleError(domain.CodeInvalid, "读取扫描图像失败"))
		return
	}
	if len(data) == 0 || int64(len(data)) > application.MaxImageBytes {
		writeError(w, domain.NewRuleError(domain.CodeInvalid, "扫描图像为空或超过 24 MiB"))
		return
	}
	expected, err := strconv.ParseInt(r.FormValue("expectedVersion"), 10, 64)
	if err != nil {
		writeError(w, domain.NewRuleError(domain.CodeInvalid, "expectedVersion 无效"))
		return
	}
	media := header.Header.Get("Content-Type")
	if media == "application/octet-stream" || media == "" {
		media = http.DetectContentType(data)
	}
	command := application.RegisterPage{ExpectedVersion: expected, FolioLabel: r.FormValue("folioLabel"), MediaType: media, Data: data, ClaimedSHA256: r.FormValue("sha256"), Actor: r.FormValue("actor")}
	volume, replayed, err := s.app.RegisterPage(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusCreated, volume, replayed)
}

func (s *Server) HandleReorderPages(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.ReorderPages
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.ReorderPages(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleUpdatePageMetadata(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.UpdatePageMetadata
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.UpdatePageMetadata(r.Context(), key, r.PathValue("volumeID"), r.PathValue("pageID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleReviseTranscription(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.ReviseTranscription
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.ReviseTranscription(r.Context(), key, r.PathValue("volumeID"), r.PathValue("pageID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandlePageImage(w http.ResponseWriter, r *http.Request) {
	reader, media, size, err := s.app.OpenPageImage(r.Context(), r.PathValue("pageID"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "private, max-age=3600, immutable")
	w.WriteHeader(http.StatusOK)
	_ = http.NewResponseController(w).Flush()
	_, _ = copyResponse(w, reader)
}
