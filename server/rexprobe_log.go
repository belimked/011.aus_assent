package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func attachProbeLog(c *rexClient, dir string) {
	path := os.Getenv("AXLEO_DB")
	if path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if db, err := sql.Open("sqlite", path); err == nil {
				if err = db.Ping(); err == nil {
					var name string
					if db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='rex_request_log'`).Scan(&name) == nil {
						c.Log = rexDBLogger(db)
						return
					}
				}
				_ = db.Close()
			}
		}
	}
	c.Log = func(row rexLog) {
		b, _ := json.Marshal(row)
		f, err := os.OpenFile(filepath.Join(dir, "rex-request-log.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Printf("Rex probe log: %v", err)
			return
		}
		defer f.Close()
		if _, err = f.Write(append(b, '\n')); err != nil {
			log.Printf("Rex probe log: %v", err)
		}
	}
}
