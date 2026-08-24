package httpui

import (
	"net/http"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
)

func (s *Server) HandleAddFinding(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.AddFinding
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.AddFinding(r.Context(), key, r.PathValue("volumeID"), r.PathValue("pageID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusCreated, volume, replayed)
}

func (s *Server) HandleResolveFinding(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.ResolveFinding
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.ResolveFinding(r.Context(), key, r.PathValue("volumeID"), r.PathValue("findingID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleIntegrityCheck(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.VersionedCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.RunIntegrityCheck(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleFreeze(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.VersionedCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.Freeze(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleAccession(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.AccessionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.Accession(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusCreated, volume, replayed)
}
