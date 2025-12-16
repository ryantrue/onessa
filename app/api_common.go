package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ryantrue/onessa/internal/logging"
)

// =============== УТИЛИТЫ ДЛЯ API ===============

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

// идентификатор пользователя для поиска соответствий между старыми и новыми
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

// логирование запросов
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &logResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)
		logging.Infof("%s %s %d %s", r.Method, r.URL.Path, lw.status, time.Since(start))
	})
}

type logResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *logResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
