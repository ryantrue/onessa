package app

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/ryantrue/onessa/internal/logging"
)

type LDAPUser struct {
	Login string
	Name  string
	Email string
}

type LDAPComputer struct {
	Name        string
	DNSHostName string
	Description string
}

func ldapUsersFilter() string {
	c := getConfig()
	if f := strings.TrimSpace(c.LDAPUsersFilter); f != "" {
		return f
	}
	return "(&(|(objectClass=user)(objectClass=person))(!(objectClass=computer))(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
}

func ldapComputersBaseDN() string {
	c := getConfig()
	if strings.TrimSpace(c.LDAPComputersBaseDN) != "" {
		return strings.TrimSpace(c.LDAPComputersBaseDN)
	}
	return ldapCfg.BaseDN
}

func ldapComputersFilter() string {
	c := getConfig()
	if f := strings.TrimSpace(c.LDAPComputersFilter); f != "" {
		return f
	}
	// Дефолт под AD: объекты компьютер+не отключённые.
	return "(&(objectClass=computer)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))"
}

func pickName(e *ldap.Entry) string {
	for _, attr := range []string{"displayName", "cn"} {
		if v := strings.TrimSpace(e.GetAttributeValue(attr)); v != "" {
			return v
		}
	}
	gn := strings.TrimSpace(e.GetAttributeValue("givenName"))
	sn := strings.TrimSpace(e.GetAttributeValue("sn"))
	if gn != "" && sn != "" {
		return gn + " " + sn
	}
	if gn != "" {
		return gn
	}
	if sn != "" {
		return sn
	}
	return ""
}

func FetchLDAPUsers(ctx context.Context) ([]LDAPUser, error) {
	if !ldapEnabled() {
		return nil, fmt.Errorf("ldap is not configured")
	}

	conn, err := ldap.DialURL(
		ldapCfg.URL,
		ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if ldapCfg.BindDN != "" {
		if err := conn.Bind(ldapCfg.BindDN, ldapCfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap bind (service): %w", err)
		}
	}

	attrs := []string{ldapCfg.UserAttribute, "mail", "displayName", "cn", "givenName", "sn"}
	req := ldap.NewSearchRequest(
		ldapCfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		ldapUsersFilter(),
		attrs,
		nil,
	)

	sr, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}

	out := make([]LDAPUser, 0, len(sr.Entries))
	for _, e := range sr.Entries {
		login := strings.TrimSpace(e.GetAttributeValue(ldapCfg.UserAttribute))
		if login == "" {
			continue
		}
		email := strings.TrimSpace(e.GetAttributeValue("mail"))
		name := pickName(e)
		out = append(out, LDAPUser{Login: login, Name: name, Email: email})
	}
	return out, nil
}

// FetchLDAPComputers читает список ПК из LDAP (обычно: объекты objectClass=computer).
func FetchLDAPComputers(ctx context.Context) ([]LDAPComputer, error) {
	if !ldapEnabled() {
		return nil, fmt.Errorf("ldap is not configured")
	}

	conn, err := ldap.DialURL(
		ldapCfg.URL,
		ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		return nil, fmt.Errorf("ldap dial: %w", err)
	}
	defer conn.Close()

	if ldapCfg.BindDN != "" {
		if err := conn.Bind(ldapCfg.BindDN, ldapCfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap bind (service): %w", err)
		}
	}

	attrs := []string{"cn", "dNSHostName", "description"}
	req := ldap.NewSearchRequest(
		ldapComputersBaseDN(),
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		ldapComputersFilter(),
		attrs,
		nil,
	)

	sr, err := conn.SearchWithPaging(req, 500)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}

	out := make([]LDAPComputer, 0, len(sr.Entries))
	for _, e := range sr.Entries {
		name := strings.TrimSpace(e.GetAttributeValue("cn"))
		if name == "" {
			continue
		}
		out = append(out, LDAPComputer{
			Name:        name,
			DNSHostName: strings.TrimSpace(e.GetAttributeValue("dNSHostName")),
			Description: strings.TrimSpace(e.GetAttributeValue("description")),
		})
	}
	return out, nil
}

// SyncLDAPUsersToDB подтягивает ВСЕХ пользователей из LDAP и делает upsert в SQLite.
// Пользователей LDAP, которых не оказалось в новой выборке, помечаем active=0 (не удаляем — чтобы не ломать назначения лицензий).
func SyncLDAPUsersToDB(ctx context.Context) (synced int, deactivated int, err error) {
	users, err := FetchLDAPUsers(ctx)
	if err != nil {
		return 0, 0, err
	}

	conn, err := requireDB()
	if err != nil {
		return 0, 0, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	beforeActive := 0
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE source='ldap' AND active=1`).Scan(&beforeActive)

	if _, err := MarkAllLDAPUsersInactive(ctx, tx); err != nil {
		return 0, 0, err
	}

	for _, u := range users {
		if err := UpsertLDAPUser(ctx, tx, u.Login, u.Name, u.Email); err != nil {
			return 0, 0, err
		}
		synced++
	}

	afterActive := 0
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE source='ldap' AND active=1`).Scan(&afterActive)
	if beforeActive > afterActive {
		deactivated = beforeActive - afterActive
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	logging.Infof("ldap sync done: synced=%d deactivated=%d", synced, deactivated)
	return synced, deactivated, nil
}

// SyncLDAPComputersToDB подтягивает ПК из LDAP и делает upsert в SQLite.
// Отсутствующие в новой выборке — помечаем active=0.
func SyncLDAPComputersToDB(ctx context.Context) (synced int, deactivated int, err error) {
	pcs, err := FetchLDAPComputers(ctx)
	if err != nil {
		return 0, 0, err
	}

	conn, err := requireDB()
	if err != nil {
		return 0, 0, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	beforeActive := 0
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM computers WHERE source='ldap' AND active=1`).Scan(&beforeActive)

	if _, err := MarkAllLDAPComputersInactive(ctx, tx); err != nil {
		return 0, 0, err
	}

	for _, pc := range pcs {
		if err := UpsertLDAPComputer(ctx, tx, pc.Name, pc.DNSHostName, pc.Description); err != nil {
			return 0, 0, err
		}
		synced++
	}

	afterActive := 0
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM computers WHERE source='ldap' AND active=1`).Scan(&afterActive)
	if beforeActive > afterActive {
		deactivated = beforeActive - afterActive
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	logging.Infof("ldap computers sync done: synced=%d deactivated=%d", synced, deactivated)
	return synced, deactivated, nil
}

// EnsureLDAPUsersLoaded — удобная обёртка для Init(): если LDAP включён, пытаемся синхронизировать.
func EnsureLDAPDataLoaded(ctx context.Context) {
	if !ldapEnabled() {
		return
	}
	if _, _, err := SyncLDAPUsersToDB(ctx); err != nil {
		logging.Warnf("ldap users sync failed: %v", err)
	}
	if _, _, err := SyncLDAPComputersToDB(ctx); err != nil {
		logging.Warnf("ldap computers sync failed: %v", err)
	}
}

// compile-time sanity
var _ = sql.ErrNoRows
