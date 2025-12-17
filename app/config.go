package app

import (
	"sync"
	"time"
)

// Config — единое место, где описаны переменные окружения.
// Парсинг делается в main.go через github.com/caarlos0/env.
type Config struct {
	// Общее
	DataDir   string `env:"DATA_DIR" envDefault:"./data"`
	HTTPAddr  string `env:"HTTP_ADDR" envDefault:":8080"`
	StaticDir string `env:"STATIC_DIR" envDefault:"./static"`

	// Логи
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`  // debug/info/warn/error
	LogFormat string `env:"LOG_FORMAT" envDefault:"text"` // text/json

	// БД
	// Если пусто — по умолчанию <DATA_DIR>/cryptopro.sqlite
	// Если относительный путь — считается относительно DATA_DIR
	DBPath string `env:"DB_PATH"`

	// Разрешённые пользователи для входа (allowlist). Нормализуются: lower + без DOMAIN\\ и @domain.
	// Если список пуст — вход разрешён всем, кто проходит LDAP-проверку.
	AuthUsers []string `env:"AUTH_USERS" envSeparator:","`

	// Сессии
	SessionSecret       string `env:"SESSION_SECRET" envDefault:"dev-insecure-secret"`
	SessionCookieSecure bool   `env:"SESSION_COOKIE_SECURE" envDefault:"false"`

	// LDAP
	LDAPURL          string `env:"LDAP_URL"`
	LDAPBaseDN       string `env:"LDAP_BASE_DN"`
	LDAPBindDN       string `env:"LDAP_BIND_DN"`
	LDAPBindPassword string `env:"LDAP_BIND_PASSWORD"`
	LDAPUserAttr     string `env:"LDAP_USER_ATTR" envDefault:"sAMAccountName"`

	// LDAP TLS
	LDAPTLSInsecureSkipVerify bool   `env:"LDAP_TLS_INSECURE_SKIP_VERIFY" envDefault:"false"`
	LDAPCAFile                string `env:"LDAP_CA_FILE"`

	// Фильтры для синхронизации (если пусто — используются дефолты для AD).
	LDAPUsersFilter     string `env:"LDAP_USERS_FILTER"`
	LDAPComputersBaseDN string `env:"LDAP_COMPUTERS_BASE_DN"`
	LDAPComputersFilter string `env:"LDAP_COMPUTERS_FILTER"`

	// Планировщик синхронизации
	LDAPSyncEvery     time.Duration `env:"LDAP_SYNC_EVERY" envDefault:"24h"`
	LDAPSyncOnStartup bool          `env:"LDAP_SYNC_ON_STARTUP" envDefault:"true"`

	// Защита write API (если задан — write /api/* без сессии разрешается только с X-API-Token)
	WriteAPIToken string `env:"WRITE_API_TOKEN"`
}

var (
	configMu sync.RWMutex
	config   Config
)

func SetConfig(c Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = c
}

func getConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}
