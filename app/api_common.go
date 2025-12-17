package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ryantrue/onessa/internal/logging"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		logging.Errorf("writeJSON error: %v", err)
	}
}

func httpError(w http.ResponseWriter, msg string, code int) {
	logging.Errorf("http error %d: %s", code, msg)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
	})
}

func userIdentity(name, email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" {
		return "email:" + email
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "" {
		return "name:" + name
	}
	return ""
}
