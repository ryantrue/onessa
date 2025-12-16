package app

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ryantrue/onessa/internal/logging"
)

// =============== ТИПЫ ЗАПРОСОВ (пользователи/лицензии) ===============

type AssignRequest struct {
	UserID    int `json:"user_id"`
	LicenseID int `json:"license_id"`
}

type UpdateLicenseRequest struct {
	LicenseID int    `json:"license_id"`
	Comment   string `json:"comment"`
	PC        string `json:"pc"`
}

type UnassignRequest struct {
	LicenseID int `json:"license_id"`
}

type ImportUsersRequest struct {
	Users []struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"users"`
}

type ImportLicensesRequest struct {
	Licenses []struct {
		Key     string `json:"key"`
		Comment string `json:"comment"`
		PC      string `json:"pc"`
	} `json:"licenses"`
}

// =============== API ОБЩЕЕ СОСТОЯНИЕ ===============

// общее состояние для фронта: список пользователей и лицензий
func handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, licenses, err := GetState(r.Context())
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		Users    []User    `json:"users"`
		Licenses []License `json:"licenses"`
	}{
		Users:    users,
		Licenses: licenses,
	}

	writeJSON(w, resp)
}

// =============== API ПОЛЬЗОВАТЕЛИ ===============

// импорт пользователей (manual fallback). Если LDAP включён — импорт отключаем.
func handleImportUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ldapEnabled() {
		httpError(w, "LDAP включён: пользователи подтягиваются из LDAP (manual import disabled)", http.StatusBadRequest)
		return
	}

	var req ImportUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Users) == 0 {
		httpError(w, "передайте хотя бы одного пользователя", http.StatusBadRequest)
		return
	}

	imported, warnings, err := ImportManualUsersUpsert(r.Context(), req.Users)
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, wmsg := range warnings {
		logging.Warnf("import users warning: %s", wmsg)
	}
	logging.Infof("import users: imported=%d warnings=%d", imported, len(warnings))

	resp := struct {
		UsersImported int      `json:"users_imported"`
		Warnings      []string `json:"warnings,omitempty"`
	}{
		UsersImported: imported,
		Warnings:      warnings,
	}

	writeJSON(w, resp)
}

// =============== API ЛИЦЕНЗИИ ===============

// импорт лицензий (всегда в БД)
func handleImportLicenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImportLicensesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Licenses) == 0 {
		httpError(w, "передайте хотя бы одну лицензию", http.StatusBadRequest)
		return
	}

	imported, warnings, err := ImportLicenses(r.Context(), req.Licenses)
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, wmsg := range warnings {
		logging.Warnf("import licenses warning: %s", wmsg)
	}
	logging.Infof("import licenses: imported=%d warnings=%d", imported, len(warnings))

	resp := struct {
		LicensesImported int      `json:"licenses_imported"`
		Warnings         []string `json:"warnings,omitempty"`
	}{
		LicensesImported: imported,
		Warnings:         warnings,
	}

	writeJSON(w, resp)
}

// привязка / перепривязка лицензии к пользователю
func handleAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.UserID == 0 || req.LicenseID == 0 {
		httpError(w, "user_id и license_id обязательны", http.StatusBadRequest)
		return
	}

	if err := AssignLicense(r.Context(), req.UserID, req.LicenseID); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "user_not_found"):
			httpError(w, "пользователь не найден", http.StatusBadRequest)
			return
		case strings.Contains(msg, "license_not_found"):
			httpError(w, "лицензия не найдена", http.StatusBadRequest)
			return
		default:
			httpError(w, "db error: "+msg, http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, map[string]any{"status": "ok"})
}

// обновление комментария и PC
func handleUpdateLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.LicenseID == 0 {
		httpError(w, "license_id обязателен", http.StatusBadRequest)
		return
	}

	if err := UpdateLicense(r.Context(), req.LicenseID, req.Comment, req.PC); err != nil {
		if strings.Contains(err.Error(), "license_not_found") {
			httpError(w, "лицензия не найдена", http.StatusBadRequest)
			return
		}
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"status": "ok"})
}

// отвязка лицензии
func handleUnassignLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UnassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "не удалось прочитать JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.LicenseID == 0 {
		httpError(w, "license_id обязателен", http.StatusBadRequest)
		return
	}

	if err := UnassignLicense(r.Context(), req.LicenseID); err != nil {
		if strings.Contains(err.Error(), "license_not_found") {
			httpError(w, "лицензия не найдена", http.StatusBadRequest)
			return
		}
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"status": "ok"})
}
