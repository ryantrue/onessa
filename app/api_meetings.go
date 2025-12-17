package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ryantrue/onessa/internal/logging"
)

type ImportMeetingsRequest struct {
	ExportedAt string    `json:"exported_at"`
	Items      []Meeting `json:"items"`
}

func handleImportMeetings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImportMeetingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		httpError(w, "передайте хотя бы одну встречу", http.StatusBadRequest)
		return
	}

	cnt, err := ReplaceMeetingsSnapshot(r.Context(), strings.TrimSpace(req.ExportedAt), req.Items)
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Infof("import meetings: imported=%d", cnt)

	resp := struct {
		Status           string `json:"status"`
		MeetingsImported int    `json:"meetings_imported"`
	}{
		Status:           "ok",
		MeetingsImported: cnt,
	}

	writeJSON(w, resp)
}

func handleMeetingsState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state, err := GetMeetingsState(r.Context())
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, state)
}
