package app

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryantrue/onessa/internal/logging"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewHTTPHandler настраивает HTTP-маршруты приложения.
func NewHTTPHandler(cfg Config) http.Handler {
	r := chi.NewRouter()

	// базовые middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(requestLogger())

	// Порядок важен:
	// 1) authMiddleware — проверка авторизации (LDAP + сессии)
	// 2) далее уже роуты (и статика)
	r.Use(authMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	// API пользователей и лицензий
	r.Route("/api", func(api chi.Router) {
		api.Get("/state", handleState)
		api.Post("/users/import", handleImportUsers)       // manual fallback
		api.Get("/users/all", handleUsersAll)              // для фронта: весь список (active + inactive)
		api.Post("/licenses/import", handleImportLicenses) // всегда в БД
		api.Post("/assign", handleAssign)
		api.Post("/license/update", handleUpdateLicense)
		api.Post("/license/unassign", handleUnassignLicense)
		api.Get("/computers", handleComputers) // список ПК из LDAP

		// API встреч
		api.Post("/meetings/import", handleImportMeetings)
		api.Get("/meetings", handleMeetingsState)
	})

	// Аутентификация
	r.Get("/login", handleLogin)
	r.Post("/login", handleLogin)
	r.Get("/logout", handleLogout)

	// Статика + SPA fallback (готово для React build в будущем).
	r.Mount("/", spaStaticHandler(cfg.StaticDir, "index.html"))

	logging.Infof("HTTP routes initialized")
	return r
}

func requestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			rid := middleware.GetReqID(r.Context())
			logging.L.WithFields(map[string]any{
				"request_id": rid,
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     ww.Status(),
				"bytes":      ww.BytesWritten(),
				"duration":   time.Since(start).String(),
				"remote":     r.RemoteAddr,
			}).Info("http_request")
		})
	}
}

func spaStaticHandler(staticDir, indexFile string) http.Handler {
	fs := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, indexFile)
	_, indexErr := os.Stat(indexPath)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		rel := path.Clean(r.URL.Path)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "." {
			rel = ""
		}
		if strings.HasPrefix(rel, "..") {
			http.NotFound(w, r)
			return
		}

		p := filepath.Join(staticDir, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}

		if indexErr != nil {
			logging.Errorf("static index not found: %s: %v", indexPath, indexErr)
			http.Error(w, "static index not found", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}
