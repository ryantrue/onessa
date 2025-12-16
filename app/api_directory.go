package app

import (
	"net/http"
)

// handleUsersAll: полный список пользователей (active+inactive).
func handleUsersAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, err := ListUsersAll(r.Context())
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, struct {
		Users []UserFull `json:"users"`
	}{Users: users})
}

// handleComputers: список ПК из LDAP, сохранённый в БД.
func handleComputers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pcs, err := ListComputers(r.Context())
	if err != nil {
		httpError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, struct {
		Computers []Computer `json:"computers"`
	}{Computers: pcs})
}
