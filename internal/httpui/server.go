package httpui

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
)

//go:embed web/*
var webAssets embed.FS

type Server struct {
	app *application.Service
	mux *http.ServeMux
}

func New(app *application.Service) *Server {
	server := &Server{app: app, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *Server) routes() {
	assets, _ := fs.Sub(webAssets, "web")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	s.mux.HandleFunc("GET /", s.HandleIndex)
	s.mux.HandleFunc("GET /healthz", s.HandleHealth)
	s.mux.HandleFunc("GET /api/volumes", s.HandleVolumes)
	s.mux.HandleFunc("POST /api/volumes", s.HandleCreateVolume)
	s.mux.HandleFunc("GET /api/volumes/{volumeID}", s.HandleWorkbench)
	s.mux.HandleFunc("PATCH /api/volumes/{volumeID}", s.HandleUpdateMetadata)
	s.mux.HandleFunc("GET /api/volumes/{volumeID}/audit", s.HandleAudit)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/pages", s.HandleRegisterPage)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/reorder", s.HandleReorderPages)
	s.mux.HandleFunc("PATCH /api/volumes/{volumeID}/pages/{pageID}", s.HandleUpdatePageMetadata)
	s.mux.HandleFunc("PUT /api/volumes/{volumeID}/pages/{pageID}/transcription", s.HandleReviseTranscription)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/pages/{pageID}/findings", s.HandleAddFinding)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/findings/{findingID}/resolve", s.HandleResolveFinding)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/checks", s.HandleIntegrityCheck)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/freeze", s.HandleFreeze)
	s.mux.HandleFunc("POST /api/volumes/{volumeID}/accession", s.HandleAccession)
	s.mux.HandleFunc("GET /api/pages/{pageID}/image", s.HandlePageImage)
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; style-src 'self'; script-src 'self'; connect-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := webAssets.ReadFile("web/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}
