-- Contact mailing lists (UAT 3 item 9), docs/rex-integration.md "联系人 Mailing lists".
-- Membership is server state: the phone sends its wanted list names in contacts.data.mailingLists,
-- the server diffs against these rows (never against the phone's copy) and queues Rex jobs.
-- status is Rex's contact_mailing_lists.subscription_status verbatim ('' = added in the app, not yet seen in Rex).
-- pending is the app change waiting for the worker ('add' / 'remove'); a poll never overwrites a pending row.
CREATE TABLE contact_mailing_lists (
 contact_id TEXT NOT NULL,
 name TEXT NOT NULL COLLATE NOCASE,
 status TEXT NOT NULL DEFAULT '',
 rex_row_id TEXT NOT NULL DEFAULT '',
 subscribed_at TEXT NOT NULL DEFAULT '',
 unsubscribed_at TEXT NOT NULL DEFAULT '',
 unsubscribe_reason TEXT NOT NULL DEFAULT '',
 pending TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL,
 PRIMARY KEY (contact_id, name)
);
-- Company catalogue from AdminMailingLists::search, refreshed by the poller (default every 6 hours).
CREATE TABLE rex_mailing_lists (
 account_id TEXT NOT NULL,
 name TEXT NOT NULL COLLATE NOCASE,
 rex_id TEXT NOT NULL DEFAULT '',
 channel TEXT NOT NULL DEFAULT '',
 data TEXT NOT NULL DEFAULT '{}',
 updated_at TEXT NOT NULL,
 PRIMARY KEY (account_id, name)
);
