package api

import "net/http"

func (s *Server) handleQueueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		s.logger.Error("queue stats", "error", err)
		writeError(w, s.logger, http.StatusInternalServerError, codeInternal, "failed to fetch queue stats")
		return
	}
	writeJSON(w, s.logger, http.StatusOK, stats)
}
