package app

import (
	"errors"
	"net/http"
	"switchyard/internal/artifacts"
)

func writeArtifactsError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadGateway, "artifacts_unavailable"
	var upstream *artifacts.Error
	if errors.As(err, &upstream) {
		status, code = upstream.Status, upstream.Code
		if upstream.RetryAfter != "" {
			w.Header().Set("Retry-After", upstream.RetryAfter)
		}
	}
	writeJSON(w, status, map[string]any{"error": code})
}
