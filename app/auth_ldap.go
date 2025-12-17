package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/ryantrue/onessa/internal/logging"
)

type LDAPConfig struct {
	URL string

	BindDN string

	BindPassword string

	BaseDN string

	UserAttribute string
}

var (
	ldapOnce sync.Once

	ldapCfg LDAPConfig
)

const (
	sessionCookieName = "cp_session"

	sessionTTL = 8 * time.Hour
)

func initLDAPConfig() {
	c := getConfig()
	ldapCfg = LDAPConfig{
		URL:          strings.TrimSpace(c.LDAPURL),
		BindDN:       strings.TrimSpace(c.LDAPBindDN),
		BindPassword: c.LDAPBindPassword,
		BaseDN:       strings.TrimSpace(c.LDAPBaseDN),
		UserAttribute: func() string {
			if strings.TrimSpace(c.LDAPUserAttr) != "" {
				return strings.TrimSpace(c.LDAPUserAttr)
			}
			return "sAMAccountName"
		}(),
	}

	if ldapCfg.URL == "" || ldapCfg.BaseDN == "" {
		logging.Warnf("LDAP is disabled: LDAP_URL / LDAP_BASE_DN are not set")
		return
	}
	logging.Infof("LDAP enabled: url=%s baseDN=%s userAttr=%s", ldapCfg.URL, ldapCfg.BaseDN, ldapCfg.UserAttribute)
}

func ldapEnabled() bool {

	ldapOnce.Do(initLDAPConfig)

	return ldapCfg.URL != "" && ldapCfg.BaseDN != ""

}

func sessionSecret() []byte {
	return []byte(getConfig().SessionSecret)

}

func makeSessionToken(username string, ts int64) string {

	payload := fmt.Sprintf("%s|%d", username, ts)

	mac := hmac.New(sha256.New, sessionSecret())

	_, _ = mac.Write([]byte(payload))

	sig := mac.Sum(nil)

	token := fmt.Sprintf("%s|%s", payload, base64.RawURLEncoding.EncodeToString(sig))

	return base64.RawURLEncoding.EncodeToString([]byte(token))

}

func parseSessionToken(token string) (string, bool) {

	raw, err := base64.RawURLEncoding.DecodeString(token)

	if err != nil {

		return "", false

	}

	parts := strings.Split(string(raw), "|")

	if len(parts) != 3 {

		return "", false

	}

	username := parts[0]

	tsStr := parts[1]

	sigB64 := parts[2]

	ts, err := strconv.ParseInt(tsStr, 10, 64)

	if err != nil {

		return "", false

	}

	issuedAt := time.Unix(ts, 0)

	if time.Since(issuedAt) > sessionTTL {

		return "", false

	}

	payload := fmt.Sprintf("%s|%d", username, ts)

	mac := hmac.New(sha256.New, sessionSecret())

	_, _ = mac.Write([]byte(payload))

	expectedSig := mac.Sum(nil)

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)

	if err != nil {

		return "", false

	}

	if !hmac.Equal(expectedSig, sig) {

		return "", false

	}

	return username, true

}

func currentUsername(r *http.Request) (string, bool) {

	c, err := r.Cookie(sessionCookieName)

	if err != nil || c.Value == "" {

		return "", false

	}

	return parseSessionToken(c.Value)

}

func setAuthCookie(w http.ResponseWriter, username string) {

	now := time.Now().Unix()

	token := makeSessionToken(username, now)

	secure := getConfig().SessionCookieSecure

	http.SetCookie(w, &http.Cookie{

		Name: sessionCookieName,

		Value: token,

		Path: "/",

		HttpOnly: true,

		SameSite: http.SameSiteLaxMode,

		Secure: secure,
	})

}

func normalizeLogin(username string) string {
	u := strings.TrimSpace(username)
	if idx := strings.Index(u, "\\"); idx != -1 && idx+1 < len(u) {
		u = u[idx+1:]
	}
	if idx := strings.Index(u, "@"); idx != -1 {
		u = u[:idx]
	}
	return strings.ToLower(strings.TrimSpace(u))
}

func authAllowed(username string) bool {
	allowed := getConfig().AuthUsers
	if len(allowed) == 0 {
		return true
	}
	u := normalizeLogin(username)
	for _, a := range allowed {
		if normalizeLogin(a) == u {
			return true
		}
	}
	return false
}

func clearAuthCookie(w http.ResponseWriter) {

	http.SetCookie(w, &http.Cookie{

		Name: sessionCookieName,

		Value: "",

		Path: "/",

		MaxAge: -1,

		HttpOnly: true,

		SameSite: http.SameSiteLaxMode,
	})

}

func staticFile(name string) string {
	dir := strings.TrimSpace(getConfig().StaticDir)
	if dir == "" {
		dir = "./static"
	}
	return filepath.Join(dir, name)
}

func ldapCheckUser(username, password string) (bool, error) {
	if !ldapEnabled() {
		return false, nil
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return false, nil
	}

	raw := strings.TrimSpace(username)
	username = normalizeLogin(raw)

	if !authAllowed(username) {
		logging.Warnf("ldapCheckUser: user %q is not in AUTH_USERS allowlist", username)
		return false, nil
	}

	logging.Infof("ldapCheckUser: start, username=%q (raw=%q)", username, raw)

	conn, err := ldap.DialURL(
		ldapCfg.URL,
		ldap.DialWithTLSConfig(ldapTLSConfig()),
	)
	if err != nil {
		logging.Errorf("ldapCheckUser: dial error: %v", err)
		return false, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if ldapCfg.BindDN != "" {
		if err := conn.Bind(ldapCfg.BindDN, ldapCfg.BindPassword); err != nil {
			logging.Errorf("ldapCheckUser: service bind error: %v", err)
			return false, fmt.Errorf("ldap bind (service): %w", err)
		}
	}

	filter := fmt.Sprintf(
		"(&(|(objectClass=user)(objectClass=person))(%s=%s))",
		ldapCfg.UserAttribute,
		ldap.EscapeFilter(username),
	)
	logging.Infof("ldapCheckUser: search baseDN=%q filter=%q", ldapCfg.BaseDN, filter)

	searchReq := ldap.NewSearchRequest(
		ldapCfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		logging.Errorf("ldapCheckUser: search error: %v", err)
		return false, fmt.Errorf("ldap search: %w", err)
	}
	if len(sr.Entries) == 0 {
		logging.Warnf("ldapCheckUser: no entries found for username=%q", username)
		return false, nil
	}

	userDN := sr.Entries[0].DN
	logging.Infof("ldapCheckUser: user %q found, DN=%q", username, userDN)

	// Проверяем пароль – bind под этим пользователем
	if err := conn.Bind(userDN, password); err != nil {
		logging.Warnf("ldapCheckUser: bad password for %q: %v", username, err)
		return false, nil
	}

	logging.Infof("ldapCheckUser: success for %q", username)
	return true, nil
}

func safeNext(next string) string {

	if next == "" {

		return "/"

	}

	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {

		return "/"

	}

	if !strings.HasPrefix(next, "/") {

		return "/"

	}

	if strings.HasPrefix(next, "//") {

		return "/"

	}

	return next

}

func redirectToLogin(w http.ResponseWriter, r *http.Request, next, errMsg string) {

	q := url.Values{}

	q.Set("next", safeNext(next))

	if errMsg != "" {

		q.Set("err", errMsg)

	}

	http.Redirect(w, r, "/login?"+q.Encode(), http.StatusFound)

}

func isPublicStaticForLogin(path string) bool {

	switch path {

	case "/layout.js", "/header.html", "/footer.html", "/favicon.ico":

		return true

	default:

		return false

	}

}

func isWriteAPIWithoutAuth(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}

	token := strings.TrimSpace(getConfig().WriteAPIToken)
	if token == "" {
		logging.Warnf("WRITE_API_TOKEN is empty: write /api/* endpoints are public (legacy mode)")
		return true
	}

	got := strings.TrimSpace(r.Header.Get("X-API-Token"))
	return got != "" && got == token
}

func authMiddleware(next http.Handler) http.Handler {

	if !ldapEnabled() {

		logging.Warnf("authMiddleware: LDAP is not configured, auth is DISABLED")

		return next

	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path := r.URL.Path

		if path == "/healthz" || path == "/login" || path == "/logout" || isPublicStaticForLogin(path) {

			next.ServeHTTP(w, r)

			return

		}

		if isWriteAPIWithoutAuth(r) {

			next.ServeHTTP(w, r)

			return

		}

		if username, ok := currentUsername(r); ok {

			logging.Infof("authMiddleware: session ok for %q, path=%s", username, path)

			next.ServeHTTP(w, r)

			return

		}

		target := r.URL.RequestURI()

		logging.Infof("authMiddleware: no session, redirecting to /login (next=%s)", target)

		redirectToLogin(w, r, target, "")

	})

}

func handleLogin(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		next := safeNext(r.URL.Query().Get("next"))

		if _, ok := currentUsername(r); ok {

			http.Redirect(w, r, next, http.StatusFound)

			return

		}

		w.Header().Set("Cache-Control", "no-store")

		http.ServeFile(w, r, staticFile("login.html"))

	case http.MethodPost:

		if err := r.ParseForm(); err != nil {

			redirectToLogin(w, r, "/", "Некорректные данные формы")

			return

		}

		username := strings.TrimSpace(r.Form.Get("username"))

		password := r.Form.Get("password")

		next := safeNext(r.Form.Get("next"))

		ok, err := ldapCheckUser(username, password)

		if err != nil {

			logging.Errorf("handleLogin: ldap error for %q: %v", username, err)

			redirectToLogin(w, r, next, "Ошибка авторизации, обратитесь к администратору")

			return

		}

		if !ok {

			redirectToLogin(w, r, next, "Неверный логин/пароль или у вас нет доступа")

			return

		}

		setAuthCookie(w, username)

		http.Redirect(w, r, next, http.StatusFound)

	default:

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

	}

}

func handleLogout(w http.ResponseWriter, r *http.Request) {

	clearAuthCookie(w)

	http.Redirect(w, r, "/login", http.StatusFound)

}
