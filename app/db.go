package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ryantrue/onessa/internal/logging"
)

// =============== МОДЕЛИ (API) ===============

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserFull нужен для фронта: полный список пользователей, включая inactive и login.
type UserFull struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Login  string `json:"login"`
	Source string `json:"source"`
	Active bool   `json:"active"`
}

type Computer struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DNSHostName string `json:"dns_host_name"`
	Description string `json:"description"`
}

type License struct {
	ID             int    `json:"id"`
	Key            string `json:"key"`
	AssignedUserID int    `json:"assigned_user_id"`
	Comment        string `json:"comment"`
	PC             string `json:"pc"`
}

type Meeting struct {
	ID           string `json:"id"`
	Subject      string `json:"subject"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Location     string `json:"location"`
	IsRecurring  bool   `json:"is_recurring"`
	IsCanceled   bool   `json:"is_canceled"`
	Link         string `json:"link"`
	Participants string `json:"participants"`
}

type MeetingsState struct {
	ExportedAt string    `json:"exported_at"`
	Items      []Meeting `json:"items"`
}

// =============== SQLite ===============

var (
	dataDir string
	db      *sql.DB
)

func SetDataDir(dir string) {
	dataDir = dir
}

// InitDB открывает SQLite (по умолчанию: <DATA_DIR>/cryptopro.sqlite) и прогоняет миграции.
func InitDB(dir string) error {
	SetDataDir(dir)

	p := strings.TrimSpace(os.Getenv("DB_PATH"))
	if p == "" {
		p = filepath.Join(dir, "cryptopro.sqlite")
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}

	// modernc.org/sqlite: driver name "sqlite"
	conn, err := sql.Open("sqlite", p)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// разумные настройки; WAL полезен для параллельных чтений
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, q := range pragmas {
		if _, e := conn.Exec(q); e != nil {
			_ = conn.Close()
			return fmt.Errorf("sqlite pragma error (%s): %w", q, e)
		}
	}

	if err := migrate(conn); err != nil {
		_ = conn.Close()
		return err
	}

	db = conn
	logging.Infof("sqlite initialized: %s", p)
	return nil
}

func migrate(conn *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			identity TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			login TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'manual',
			active INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_users_active ON users(active);`,
		`CREATE TABLE IF NOT EXISTS computers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			identity TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			dns_host_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'manual',
			active INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_computers_active ON computers(active);`,
		`CREATE TABLE IF NOT EXISTS licenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			assigned_user_id INTEGER NULL,
			comment TEXT NOT NULL DEFAULT '',
			pc TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(assigned_user_id) REFERENCES users(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS meetings_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			exported_at TEXT NOT NULL DEFAULT ''
		);`,
		`INSERT OR IGNORE INTO meetings_meta(id, exported_at) VALUES (1, '');`,
		`CREATE TABLE IF NOT EXISTS meetings (
			id TEXT PRIMARY KEY,
			subject TEXT NOT NULL DEFAULT '',
			start TEXT NOT NULL DEFAULT '',
			end TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			is_recurring INTEGER NOT NULL DEFAULT 0,
			is_canceled INTEGER NOT NULL DEFAULT 0,
			link TEXT NOT NULL DEFAULT '',
			participants TEXT NOT NULL DEFAULT ''
		);`,
	}

	for _, s := range stmts {
		if _, err := conn.Exec(s); err != nil {
			return fmt.Errorf("sqlite migrate error: %w (sql=%s)", err, s)
		}
	}
	return nil
}

func requireDB() (*sql.DB, error) {
	if db == nil {
		return nil, errors.New("db is not initialized")
	}
	return db, nil
}

// =============== USERS ===============

func ListUsers(ctx context.Context) ([]User, error) {
	conn, err := requireDB()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id, name, email FROM users WHERE active=1 ORDER BY name, email, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListUsersAll отдаёт полный список пользователей (active + inactive),
// чтобы фронт мог показывать историю/старые привязки.
func ListUsersAll(ctx context.Context) ([]UserFull, error) {
	conn, err := requireDB()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, name, email, login, source, active
		FROM users
		ORDER BY active DESC, name, email, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserFull
	for rows.Next() {
		var u UserFull
		var activeInt int
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Login, &u.Source, &activeInt); err != nil {
			return nil, err
		}
		u.Active = activeInt != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListComputers отдаёт список ПК из БД (active=1).
func ListComputers(ctx context.Context) ([]Computer, error) {
	conn, err := requireDB()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, name, dns_host_name, description
		FROM computers
		WHERE active=1
		ORDER BY name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Computer
	for rows.Next() {
		var c Computer
		if err := rows.Scan(&c.ID, &c.Name, &c.DNSHostName, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ImportManualUsersUpsert — импорт/обновление пользователей из JSON (fallback, когда LDAP не настроен).
func ImportManualUsersUpsert(ctx context.Context, in []struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}) (imported int, warnings []string, err error) {
	conn, err := requireDB()
	if err != nil {
		return 0, nil, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO users(identity, name, email, login, source, active, updated_at)
		VALUES(?, ?, ?, '', 'manual', 1, ?)
		ON CONFLICT(identity) DO UPDATE SET
			name=excluded.name,
			email=excluded.email,
			active=1,
			updated_at=excluded.updated_at
	`)
	if err != nil {
		return 0, nil, err
	}
	defer stmt.Close()

	for _, u := range in {
		name := strings.TrimSpace(u.Name)
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if name == "" && email == "" {
			warnings = append(warnings, "пропущена запись без имени и email")
			continue
		}

		identity := userIdentity(name, email)
		if identity == "" {
			warnings = append(warnings, "не удалось сформировать identity для пользователя "+name)
			continue
		}

		if _, e := stmt.ExecContext(ctx, identity, name, email, now); e != nil {
			err = e
			return 0, warnings, err
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, warnings, err
	}
	return imported, warnings, nil
}

// UpsertLDAPUser — внутренняя утилита для LDAP-синхронизации.
func UpsertLDAPUser(ctx context.Context, tx *sql.Tx, login, name, email string) error {
	login = strings.TrimSpace(login)
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if login == "" {
		return errors.New("empty ldap login")
	}
	identity := "ldap:" + strings.ToLower(login)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users(identity, name, email, login, source, active, updated_at)
		VALUES(?, ?, ?, ?, 'ldap', 1, ?)
		ON CONFLICT(identity) DO UPDATE SET
			name=excluded.name,
			email=excluded.email,
			login=excluded.login,
			active=1,
			updated_at=excluded.updated_at
	`, identity, name, email, login, now)
	return err
}

// MarkAllLDAPUsersInactive — перед синхронизацией: деактивируем всех LDAP-пользователей.
func MarkAllLDAPUsersInactive(ctx context.Context, tx *sql.Tx) (int, error) {
	res, err := tx.ExecContext(ctx, `UPDATE users SET active=0 WHERE source='ldap'`)
	if err != nil {
		return 0, err
	}
	a, _ := res.RowsAffected()
	return int(a), nil
}

// UpsertLDAPComputer — внутренняя утилита для LDAP-синхронизации ПК.
func UpsertLDAPComputer(ctx context.Context, tx *sql.Tx, name, dnsHostName, description string) error {
	name = strings.TrimSpace(name)
	dnsHostName = strings.TrimSpace(dnsHostName)
	description = strings.TrimSpace(description)
	if name == "" {
		return errors.New("empty computer name")
	}
	identity := "ldap:" + strings.ToLower(name)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, `
		INSERT INTO computers(identity, name, dns_host_name, description, source, active, updated_at)
		VALUES(?, ?, ?, ?, 'ldap', 1, ?)
		ON CONFLICT(identity) DO UPDATE SET
			name=excluded.name,
			dns_host_name=excluded.dns_host_name,
			description=excluded.description,
			active=1,
			updated_at=excluded.updated_at
	`, identity, name, dnsHostName, description, now)
	return err
}

// MarkAllLDAPComputersInactive — перед синхронизацией: деактивируем все LDAP-компьютеры.
func MarkAllLDAPComputersInactive(ctx context.Context, tx *sql.Tx) (int, error) {
	res, err := tx.ExecContext(ctx, `UPDATE computers SET active=0 WHERE source='ldap'`)
	if err != nil {
		return 0, err
	}
	a, _ := res.RowsAffected()
	return int(a), nil
}

// =============== LICENSES ===============

func ListLicenses(ctx context.Context) ([]License, error) {
	conn, err := requireDB()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id, key, assigned_user_id, comment, pc FROM licenses ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []License
	for rows.Next() {
		var l License
		var assigned sql.NullInt64
		if err := rows.Scan(&l.ID, &l.Key, &assigned, &l.Comment, &l.PC); err != nil {
			return nil, err
		}
		if assigned.Valid {
			l.AssignedUserID = int(assigned.Int64)
		} else {
			l.AssignedUserID = 0
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func ImportLicenses(ctx context.Context, in []struct {
	Key     string `json:"key"`
	Comment string `json:"comment"`
	PC      string `json:"pc"`
}) (imported int, warnings []string, err error) {
	conn, err := requireDB()
	if err != nil {
		return 0, nil, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO licenses(key, assigned_user_id, comment, pc, created_at) VALUES(?, NULL, ?, ?, ?);`)
	if err != nil {
		return 0, nil, err
	}
	defer stmt.Close()

	for _, lic := range in {
		key := strings.TrimSpace(lic.Key)
		if key == "" {
			warnings = append(warnings, "пропущена лицензия без ключа")
			continue
		}
		if _, e := stmt.ExecContext(ctx, key, strings.TrimSpace(lic.Comment), strings.TrimSpace(lic.PC), now); e != nil {
			if isUniqueConstraintError(e) {
				warnings = append(warnings, "дубликат ключа: "+key)
				continue
			}
			err = e
			return 0, warnings, err
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, warnings, err
	}
	return imported, warnings, nil
}

func AssignLicense(ctx context.Context, userID, licenseID int) error {
	conn, err := requireDB()
	if err != nil {
		return err
	}

	// проверяем пользователя
	var tmp int
	if err := conn.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id=? AND active=1`, userID).Scan(&tmp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user_not_found")
		}
		return err
	}

	res, err := conn.ExecContext(ctx, `UPDATE licenses SET assigned_user_id=? WHERE id=?`, userID, licenseID)
	if err != nil {
		return err
	}
	a, _ := res.RowsAffected()
	if a == 0 {
		return fmt.Errorf("license_not_found")
	}
	return nil
}

func UpdateLicense(ctx context.Context, licenseID int, comment, pc string) error {
	conn, err := requireDB()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `UPDATE licenses SET comment=?, pc=? WHERE id=?`, strings.TrimSpace(comment), strings.TrimSpace(pc), licenseID)
	if err != nil {
		return err
	}
	a, _ := res.RowsAffected()
	if a == 0 {
		return fmt.Errorf("license_not_found")
	}
	return nil
}

func UnassignLicense(ctx context.Context, licenseID int) error {
	conn, err := requireDB()
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `UPDATE licenses SET assigned_user_id=NULL WHERE id=?`, licenseID)
	if err != nil {
		return err
	}
	a, _ := res.RowsAffected()
	if a == 0 {
		return fmt.Errorf("license_not_found")
	}
	return nil
}

// =============== MEETINGS ===============

func ReplaceMeetingsSnapshot(ctx context.Context, exportedAt string, items []Meeting) (int, error) {
	conn, err := requireDB()
	if err != nil {
		return 0, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `UPDATE meetings_meta SET exported_at=? WHERE id=1`, strings.TrimSpace(exportedAt)); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM meetings`); err != nil {
		return 0, err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO meetings(id, subject, start, end, location, is_recurring, is_canceled, link, participants)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	cnt := 0
	for _, m := range items {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		if _, err := stmt.ExecContext(
			ctx,
			strings.TrimSpace(m.ID),
			strings.TrimSpace(m.Subject),
			strings.TrimSpace(m.Start),
			strings.TrimSpace(m.End),
			strings.TrimSpace(m.Location),
			boolToInt(m.IsRecurring),
			boolToInt(m.IsCanceled),
			strings.TrimSpace(m.Link),
			strings.TrimSpace(m.Participants),
		); err != nil {
			return 0, err
		}
		cnt++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return cnt, nil
}

func GetMeetingsState(ctx context.Context) (MeetingsState, error) {
	conn, err := requireDB()
	if err != nil {
		return MeetingsState{}, err
	}

	var exportedAt string
	if err := conn.QueryRowContext(ctx, `SELECT exported_at FROM meetings_meta WHERE id=1`).Scan(&exportedAt); err != nil {
		return MeetingsState{}, err
	}

	rows, err := conn.QueryContext(ctx, `
		SELECT id, subject, start, end, location, is_recurring, is_canceled, link, participants
		FROM meetings ORDER BY start, id
	`)
	if err != nil {
		return MeetingsState{}, err
	}
	defer rows.Close()

	var items []Meeting
	for rows.Next() {
		var m Meeting
		var rec, canc int
		if err := rows.Scan(&m.ID, &m.Subject, &m.Start, &m.End, &m.Location, &rec, &canc, &m.Link, &m.Participants); err != nil {
			return MeetingsState{}, err
		}
		m.IsRecurring = rec != 0
		m.IsCanceled = canc != 0
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return MeetingsState{}, err
	}

	return MeetingsState{ExportedAt: exportedAt, Items: items}, nil
}

// =============== STATE ===============

func GetState(ctx context.Context) (users []User, licenses []License, err error) {
	users, err = ListUsers(ctx)
	if err != nil {
		return nil, nil, err
	}
	licenses, err = ListLicenses(ctx)
	if err != nil {
		return nil, nil, err
	}
	return users, licenses, nil
}

// =============== helpers ===============

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// modernc + sqlite обычно пишут "constraint failed" / "unique".
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}
