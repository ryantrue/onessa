package app

import (
	"sync"
	"time"
)

// Config — единое место, где описаны переменные окружения.
// Парсинг делается в main.go через github.com/caarlos0/env.
type Config struct {
	// Общее
	DataDir  string `env:"DATA_DIR" envDefault:"./data"`
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`

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

	// Фильтры для синхронизации (если пусто — используются дефолты для AD).
	LDAPUsersFilter     string `env:"LDAP_USERS_FILTER"`
	LDAPComputersBaseDN string `env:"LDAP_COMPUTERS_BASE_DN"`
	LDAPComputersFilter string `env:"LDAP_COMPUTERS_FILTER"`

	// Планировщик синхронизации
	LDAPSyncEvery     time.Duration `env:"LDAP_SYNC_EVERY" envDefault:"24h"`
	LDAPSyncOnStartup bool          `env:"LDAP_SYNC_ON_STARTUP" envDefault:"true"`
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
