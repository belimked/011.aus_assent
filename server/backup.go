package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// A separate, persistent lock survives atomic database replacement. Serve holds it
// for its lifetime; restore must acquire it before touching the database files.
func serviceLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("database in use; stop axleo.service before restoring or resetting data: %w", err)
	}
	return f, nil
}
func backup(db *sql.DB, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	file := filepath.Join(dir, fmt.Sprintf("axleo-%s-%d.db", time.Now().UTC().Format("20060102-150405"), time.Now().UnixNano()))
	_, err := db.Exec(`VACUUM INTO '` + strings.ReplaceAll(file, "'", "''") + `'`)
	if err != nil {
		os.Remove(file)
		return "", err
	}
	return file, nil
}
func verify(path string) (map[string]int64, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var result string
	if err = db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return nil, err
	}
	if result != "ok" {
		return nil, fmt.Errorf("integrity_check: %s", result)
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for rows.Next() {
		var n string
		if err = rows.Scan(&n); err != nil {
			break
		}
		names = append(names, n)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	supported, err := assets.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	hasMigrations := false
	for _, n := range names {
		if n == "schema_migrations" {
			hasMigrations = true
		}
	}
	if !hasMigrations {
		return nil, fmt.Errorf("missing schema_migrations")
	}
	versions, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	count := 0
	for versions.Next() {
		var v string
		if err = versions.Scan(&v); err != nil {
			break
		}
		count++
		found := false
		for _, e := range supported {
			if e.Name() == v {
				found = true
				break
			}
		}
		if !found {
			err = fmt.Errorf("unsupported schema version %s", v)
			break
		}
	}
	if e := versions.Err(); err == nil {
		err = e
	}
	versions.Close()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("empty schema")
	}
	sort.Strings(names)
	counts := map[string]int64{}
	for _, n := range names {
		var c int64
		if err = db.QueryRow(`SELECT count(*) FROM "` + strings.ReplaceAll(n, `"`, `""`) + `"`).Scan(&c); err != nil {
			return nil, err
		}
		counts[n] = c
	}
	return counts, nil
}
func restore(path, source string) (string, error) {
	absPath, _ := filepath.Abs(path)
	absSource, _ := filepath.Abs(source)
	if absPath == absSource {
		return "", fmt.Errorf("source and destination must differ")
	}
	lock, err := serviceLock(path)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if _, err = verify(source); err != nil {
		return "", err
	}
	db, err := open(path)
	if err != nil {
		return "", err
	}
	// Do not migrate or mutate the old database while restoring.
	saved, err := backup(db, filepath.Join(filepath.Dir(path), "backup"))
	db.Close()
	if err != nil {
		return "", err
	}
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".restore-*.db")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.ReadFrom(input); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if _, err = verify(tmp.Name()); err != nil {
		return "", err
	}
	// WAL may contain committed changes from the old database; never replay it onto the snapshot.
	for _, suffix := range []string{"-wal", "-shm"} {
		if err = os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return saved, nil
}
