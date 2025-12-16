package app

import (
	"context"
	"net/http"

	"github.com/ryantrue/onessa/internal/logging"
)

// Init настраивает SQLite и фоновые задачи (LDAP sync).
func Init(ctx context.Context, cfg Config) error {
	SetConfig(cfg)
	if err := InitDB(cfg.DataDir); err != nil {
		return err
	}
	// Пользователи/компьютеры в системе должны быть загружены из LDAP.
	// Если LDAP не настроен — проект может работать в "manual" режиме (импорт пользователей через API).
	EnsureLDAPDataLoaded(ctx)
	StartBackgroundLDAPSync(ctx)
	return nil
}

// NewHTTPHandler настраивает HTTP-маршруты приложения.
func NewHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	// API пользователей и лицензий
	mux.HandleFunc("/api/state", handleState)
	mux.HandleFunc("/api/users/import", handleImportUsers)       // manual fallback
	mux.HandleFunc("/api/users/all", handleUsersAll)             // для фронта: весь список (active + inactive)
	mux.HandleFunc("/api/licenses/import", handleImportLicenses) // всегда в БД
	mux.HandleFunc("/api/assign", handleAssign)
	mux.HandleFunc("/api/license/update", handleUpdateLicense)
	mux.HandleFunc("/api/license/unassign", handleUnassignLicense)
	mux.HandleFunc("/api/computers", handleComputers) // список ПК из LDAP

	// API встреч
	mux.HandleFunc("/api/meetings/import", handleImportMeetings)
	mux.HandleFunc("/api/meetings", handleMeetingsState)

	// Аутентификация (страница логина и logout)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/logout", handleLogout)

	// статика (index.html, import.html, meetings.html, js, css)
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	logging.Infof("HTTP routes initialized")

	// Порядок важен:
	// 1) authMiddleware — проверка авторизации (LDAP + сессии)
	// 2) loggingMiddleware — логируем уже авторизованные запросы
	return loggingMiddleware(authMiddleware(mux))
}
