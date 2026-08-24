package httpui

import (
	"net/http"

	"benzhi-project-c7cf8217-f846-4eb7-857c-295b5cacde7f/internal/application"
)

func (s *Server) HandleVolumes(w http.ResponseWriter, r *http.Request) {
	volumes, err := s.app.ListVolumes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, responseEnvelope{Data: volumes})
}

func (s *Server) HandleCreateVolume(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.CreateVolume
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.CreateVolume(r.Context(), key, command)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeResult(w, status, volume, replayed)
}

func (s *Server) HandleWorkbench(w http.ResponseWriter, r *http.Request) {
	volumeID, _ := pathID(r, "volumeID")
	view, err := s.app.GetWorkbench(r.Context(), volumeID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, responseEnvelope{Data: view})
}

func (s *Server) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	key, err := requireIdempotency(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.UpdateMetadata
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	volume, replayed, err := s.app.UpdateMetadata(r.Context(), key, r.PathValue("volumeID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeResult(w, http.StatusOK, volume, replayed)
}

func (s *Server) HandleAudit(w http.ResponseWriter, r *http.Request) {
	events, err := s.app.ListAudit(r.Context(), r.PathValue("volumeID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, responseEnvelope{Data: events})
}
