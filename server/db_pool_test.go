package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testReaders opens the production read pool on a test database file. Tests use
// files (t.TempDir), never :memory:, so the writer and the pool share data.
func testReaders(t testing.TB, path string) *sql.DB {
	t.Helper()
	rdb, err := openReaders(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

// appConnGuard is singleConnGuard for an app: the writer must not deadlock or
// leak, and no read-pool connection may stay checked out afterwards either.
func appConnGuard(t *testing.T, a *app, name string, fn func() error) {
	t.Helper()
	singleConnGuard(t, a.db, name, fn)
	if a.rdb != nil {
		if s := a.rdb.Stats(); s.InUse != 0 {
			t.Fatalf("%s left %d read-pool connection(s) checked out", name, s.InUse)
		}
	}
}

func TestReadPoolShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pool.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db, path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AXLEO_DB_READERS", "6")
	if n := readerCount(); n != 6 {
		t.Fatal("AXLEO_DB_READERS", n)
	}
	t.Setenv("AXLEO_DB_READERS", "junk")
	if n := readerCount(); n != 4 {
		t.Fatal("default readers", n)
	}
	rdb := testReaders(t, path)
	if n := rdb.Stats().MaxOpenConnections; n != 4 {
		t.Fatal("pool size", n)
	}
	// Every pooled connection (not just the first) is read-only and waits on locks.
	conns := make([]*sql.Conn, 4)
	for i := range conns {
		if conns[i], err = rdb.Conn(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for i, c := range conns {
		var qo, busy, fk int
		if err = c.QueryRowContext(context.Background(), `PRAGMA query_only`).Scan(&qo); err != nil {
			t.Fatal(err)
		}
		_ = c.QueryRowContext(context.Background(), `PRAGMA busy_timeout`).Scan(&busy)
		_ = c.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&fk)
		if qo != 1 || busy != busyTimeoutMS || fk != 1 {
			t.Fatalf("conn %d: query_only=%d busy_timeout=%d foreign_keys=%d", i, qo, busy, fk)
		}
		if _, err = c.ExecContext(context.Background(), `INSERT INTO settings(key,value) VALUES('x','y')`); err == nil || !strings.Contains(strings.ToLower(err.Error()), "readonly") {
			t.Fatalf("conn %d accepted a write: %v", i, err)
		}
	}
	for _, c := range conns {
		c.Close()
	}
	// A pool on a rollback-journal database would block the writer: refuse it.
	plain := filepath.Join(t.TempDir(), "plain.db")
	pdb, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer pdb.Close()
	if _, err = pdb.Exec(`CREATE TABLE t(x)`); err != nil {
		t.Fatal(err)
	}
	if _, err = openReaders(plain, 2); err == nil {
		t.Fatal("read pool opened on a non-WAL database")
	}
}

// Readers see committed writes immediately; a read snapshot keeps its view for
// its whole life; a long snapshot does not block the writer.
func TestReadPoolConsistency(t *testing.T) {
	a, _ := setup(t)
	var before int
	if err := a.rd().QueryRow(`SELECT count(*) FROM teams`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	snap, done, err := a.readSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err = snap.QueryRow(`SELECT count(*) FROM teams`).Scan(&n); err != nil || n != before {
		t.Fatal(n, err)
	}
	finished := make(chan error, 1)
	go func() {
		_, e := a.db.Exec(`INSERT INTO teams(id,name) VALUES('t-new','New')`)
		finished <- e
	}()
	select {
	case e := <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writer blocked by an open read snapshot")
	}
	if err = snap.QueryRow(`SELECT count(*) FROM teams`).Scan(&n); err != nil || n != before {
		t.Fatalf("snapshot saw a later commit: %d %v", n, err)
	}
	done()
	if err = a.rd().QueryRow(`SELECT count(*) FROM teams`).Scan(&n); err != nil || n != before+1 {
		t.Fatalf("pool did not see the committed write: %d %v", n, err)
	}
	// Inside an open write transaction the pool sees only committed data: reads
	// that must include the transaction's own rows have to use the tx.
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO teams(id,name) VALUES('t-tx','Tx')`); err != nil {
		t.Fatal(err)
	}
	var inPool, inTx int
	_ = a.rd().QueryRow(`SELECT count(*) FROM teams WHERE id='t-tx'`).Scan(&inPool)
	_ = tx.QueryRow(`SELECT count(*) FROM teams WHERE id='t-tx'`).Scan(&inTx)
	_ = tx.Rollback()
	if inPool != 0 || inTx != 1 {
		t.Fatalf("pool=%d tx=%d", inPool, inTx)
	}
}

// Without a pool (CLI commands, older fixtures) reads fall back to the writer
// and a "snapshot" must not hold the single connection (nested reads would hang).
func TestReadSnapshotWithoutPool(t *testing.T) {
	a, _ := setup(t)
	a = &app{db: a.db}
	if a.rd() != querier(a.db) {
		t.Fatal("rd must fall back to the writer")
	}
	singleConnGuard(t, a.db, "snapshot fallback", func() error {
		snap, done, err := a.readSnapshot(context.Background())
		if err != nil {
			return err
		}
		defer done()
		rows, err := snap.Query(`SELECT id FROM users`)
		if err != nil {
			return err
		}
		rows.Close()
		// A real transaction would still hold the only connection here.
		var n int
		return a.db.QueryRow(`SELECT count(*) FROM teams`).Scan(&n)
	})
}

// Many concurrent pulls and searches share the pool without errors.
func TestReadPoolConcurrentRequests(t *testing.T) {
	a, tok := setup(t)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/api/v1/sync"
			if i%2 == 1 {
				path = "/api/v1/contacts/search?q=mary"
			}
			if w := request(a, "GET", path, tok["mary"], nil); w.Code != 200 {
				errs <- w.Body.String()
			}
			if w := request(a, "POST", "/api/v1/contacts", tok["mary"], map[string]any{"first": "C"}); w.Code != 201 {
				errs <- w.Body.String()
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
