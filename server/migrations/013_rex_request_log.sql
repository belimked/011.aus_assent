CREATE TABLE rex_request_log (
 id INTEGER PRIMARY KEY, at TEXT NOT NULL, account_id TEXT NOT NULL,
 direction TEXT NOT NULL CHECK(direction IN ('out','in')),
 kind TEXT NOT NULL CHECK(kind IN ('auth','read','write')),
 service TEXT NOT NULL, method TEXT NOT NULL, path TEXT NOT NULL,
 caller TEXT NOT NULL, attempt INTEGER NOT NULL,
 request_json TEXT NOT NULL, http_status INTEGER NOT NULL, ok INTEGER NOT NULL,
 error TEXT NOT NULL, duration_ms INTEGER NOT NULL,
 result_summary TEXT NOT NULL, response_excerpt TEXT NOT NULL
);
CREATE INDEX rex_request_log_at ON rex_request_log(at);
CREATE INDEX rex_request_log_service ON rex_request_log(service);
CREATE INDEX rex_request_log_kind ON rex_request_log(kind);
