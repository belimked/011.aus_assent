# Contact merge

Merging two duplicate contacts takes two steps: the user previews the result, then confirms. No admin approval is involved, and **a merge cannot be undone**. The audit log keeps both records exactly as they were before the merge, so a mistake can be fixed by hand.

## Who may merge

| Role | Allowed |
|---|---|
| Agent | Two contacts that are both visible to them and that they own |
| Admin | Any two contacts in the team |

If either contact is not visible, or does not belong to the agent, the server returns `404`, the same as for any other hidden record. Merging needs a connection: the app does not queue merges offline, and it does not merge locally.

## API

### `POST /api/v1/contacts/merge-preview`

```json
{"aId": "c1", "bId": "c2"}
```

The order of `aId` and `bId` does not matter. The response is the full merged result, with nothing written:

| Field | Meaning |
|---|---|
| `primaryId` / `secondaryId` | The record kept (A) and the record removed (B) |
| `primaryReason` | `rex` (A is the only one linked to Rex) or `older` (A was created first) |
| `contacts` | Summaries of `[A, B]` |
| `fields` | Each scalar field: `value`, `from` (contact id), `a`, `b`, `reason` (`newer`, `only` or `same`) |
| `dnc` | `{value, a, b}` |
| `consent` | Each channel: `{channel, value, a, b}` |
| `notes` | Combined notes, newest first. The first entry is the merge log note (`log: true`) |
| `tags`, `roles` | Combined lists |
| `properties` | Each property relation: `relation`, `label`, `at`, `from`, `changed`, `replaced[]` |
| `relations` | The merged `rels` JSON |
| `logNote` | The note added to A, in English and Chinese |
| `rex` | `both`, `primary`, `secondary`, `none`, or `pending` (a Rex create is still queued) |
| `previewHash` | Fingerprint of everything shown |

### `POST /api/v1/contacts/{primaryId}/merge`

```json
{"secondaryId": "c2", "previewHash": "…"}
```

- **`200`** returns `{ok, primaryId, secondaryId, contact}`. The app then opens A.
- **`409`** means `previewHash` is missing or does not match the current data, because someone edited either record or a related row since the preview. The body includes a fresh `preview`. Nothing is written. The app shows the new preview and asks the user to confirm again.
- The primary is always chosen by the rules below, whichever of the two ids is in the path.
- **`404`** means a contact is missing, deleted (including already merged), or not allowed for this user.

`previewHash` covers both records' `id`, `version` and `updated_at`, the merged result, and the source note and relation rows. The dated log note is deliberately left out, so a preview stays valid as long as nothing changes. The execute step recomputes the plan inside a single transaction and checks the hash there, so what the user saw is exactly what gets written.

### Legacy approvals

The `approvals` table and `/api/v1/approvals` endpoints still work. When an admin approves a pending `merge` request created before this change, it runs through the same `executeMerge` with these rules (there is no hash, because the admin is the one deciding). The app no longer creates merge requests, but existing approval cards still show.

## Rules

### Which record is kept (primary)

1. If exactly one record is linked to Rex, that one is kept.
2. Otherwise (both or neither are linked), the one created first is kept (`created_at`; the lower id breaks a tie).

The primary only decides **which id survives**. Field values are chosen by the timeline rules below, whichever record is primary.

### Scalar fields: newest value wins

Applies to `first`, `last`, `mobile`, `email`, `address`, `reason`, and every other scalar key in `data`.

- **Time source:** `data.fieldTimes[key]` if the record has a time for that field, otherwise the record's `updated_at`.
- The value with the later time wins. On a tie, the primary's value wins.
- **A blank value never overwrites.** If the newer side is empty (`""`, `null`, an empty list or object), the other side's value is kept.
- The contact columns (`first`, `last`, `mobile`, `email`, `roles`, `tags`, `flags`, `consent`) are rewritten from the merged `data`.
- Unknown list fields (for example history) are combined, with exact duplicates removed.

### Compliance exceptions

These ignore time:

- **DNC:** if either side is DNC, the result is DNC.
- **Consent, per channel:** the stricter value wins. If either side refused (`0` or `false`), the result is refused. `optedOut` follows the merged SMS consent.

### Notes

- Sources are `data.notes` and `contact_notes` rows from both records.
- Exact duplicates (same date, author and text after trimming) are kept once.
- Ordering is newest first, matching the app's `notes[0]`. The sort time is the `contact_notes.created_at` when present, otherwise the note date.
- B's `contact_notes` rows move to A.
- A merge log note is added to the top and inserted into `contact_notes`:

  ```
  Merged with Ann Lee (0411 111 111) by Mary on 2026-05-02. Relation to 3 Feltwell St, Sunnybank changed from Tenant to Owner by merge.
  由 Mary 于 2026-05-02 与 Ann Lee (0411 111 111) 合并。与 3 Feltwell St, Sunnybank 的关系因合并由 Tenant 改为 Owner。
  ```

### Tags and roles

- **Tags:** union, deduplicated ignoring case and surrounding spaces. When two tags match, the spelling from the record with the newer `tags` time is kept.
- **Roles:** union, deduplicated ignoring case.

### Property relations

**Sources:**
- `property_owners` rows, timed by the row's `updated_at`.
- `rels` entries with a `propertyId`, timed by the record's `rels` field time.

**Resolution:**
- A property only one side relates to: the relation is appended. The result is ordered by time.
- The same property with the same relation type: kept once.
- The same property with different relation types: the latest wins. Each superseded type is written into the log note ("Relation to … changed from Tenant to Owner by merge").
- Afterwards, `property_owners` has exactly one row per property for A, keeping the winning time. Every affected property gets a version bump so phones pull the change.
- `rels` entries with a `propertyId` are rewritten to the winning type, so `rels` and `property_owners` never disagree.
- Contact-to-contact relations are combined. References to B become A, and any self-reference is dropped.

#### `property_owners.updated_at` (migration 019)

| Event | Value |
|---|---|
| Migration backfill | The contact's `updated_at`, falling back to the property's `updated_at` |
| Insert without a time | Stamped with the current time (trigger) |
| Relation type changes and the writer did not set a time | Stamped with the current time (trigger) |
| Property PATCH from the phone (`ownerIds`) | Only removed contacts are deleted. Existing rows keep their relation type and time. New rows are stamped with the current time. |
| Rex import | An unchanged relation keeps its time |

### Other references

- **Moved to A:** `todos`, `leads`, `ofi_attendees`, `referrals`. Both the `contact_id` columns and the JSON fields are updated. Other contacts' `rels` that point at B are also repointed.
- B is soft-deleted (`deleted_at`), so phones receive a tombstone.
- `audit_log` gets two rows: `merge` on A and `merged-into` on B. Both contain the full before-snapshots of the two records (data, notes rows, property rows, Rex link) and the plan.

### Possible-duplicate flags (`data.possibleDuplicateOf`)

The same-mobile dedupe (docs/rex-integration.md, 同号码查重) flags contacts with `data.possibleDuplicateOf`. A merge clears only the merged pair:

- A gets both lists combined, minus A and B. Flags that name a third contact stay. If nothing is left, the key is removed.
- B's flags are removed (they now live on A).
- A third contact that flagged B now flags A instead, and its version is bumped so phones receive the change.

### Poller after a merge

- When the link has moved to A (only B was linked), the poller finds A through `rex_links` and updates A. It never imports a second contact.
- Until `rex_merge` has run, the link still names the merged-away B. The poller skips such a contact (`mergedAway`: soft-deleted with a `merged-into` audit row). Otherwise the import would clear B's `deleted_at` and bring it back.

### Search results

Contact search shows other agents' contacts only in the limited view. Merge preview and merge return `404` for any contact the caller cannot read, so a limited search result can never be merged. A Rex-only result has to be imported first (`import-rex`, which makes the caller the owner).

## Rex

The local merge commits first. Rex work runs afterwards through the outbox (`rex_merge`) and follows the normal authorisation and retry rules.

| Rex state at merge | What happens |
|---|---|
| Both linked | `Dedupe::combineRecords` (`winning_id` = A, `losing_ids` = [B]). Then A is read back and corrected (see below), DNC is set if needed, relations are aligned, and the log note is created on A. |
| Only B linked | B's `rex_links` row is moved to A, then A is updated as above. |
| Only A linked | A is updated only. |
| Neither linked | No Rex job. |
| A create is still queued | The job waits for the link, for at most 10 minutes (`mergeLinkWait`). After that it is skipped with "timed out waiting for the Rex contact link". Legacy jobs without a deadline use the outbox `created_at`. |

**What `combineRecords` does (sandbox 2057, 2026-11-26):**
- Response: `{"result":[{"losing_id":"B","records_reassigned":[{"service_name":"Notes","record_ids":[…]}]}]}`. It is synchronous: B is `trashed` immediately.
- A's name becomes both names joined: "AXLEO MERGE TEST A & AXLEO MERGE TEST B".
- All of B's phones, emails and tags are appended to A. Nothing is de-duplicated; B's phone arrives with `phone_primary = null`.
- B's notes move to A (`note_contacts` now A).
- Property both were related to (23138536: A owner row 17077130, B tenant row 17077142): one row is left, **id 17077130 (A's), contact A, type tenant (B's)**. B's row is gone. So Rex keeps the winner's row id but takes the loser's relation type.
- Not measured: B's rows on properties A had no relation to. The worker does not assume anything; it reads A's actual rows.
- `Properties::update` with `contact_reln_property: [{id: 17077130, _destroy: true}]` removes that row only; other rows (Teddie Bear owner 17016203) are untouched.

**Contact correction (`contacts` PATCH job with `merge: true`, `stepMergeContact`):**
1. `Contacts::read` A (with `related`).
2. One `Contacts::update`:
   - Name: the first `contact_names` row gets the local merged first/last; any other name rows are sent as `{id, _destroy: true}`.
   - Phones: grouped by `normAUMobile`; every duplicate row is destroyed. The row of the local merged mobile gets `phone_primary: true`, any other primary row gets `false`. If the local mobile is missing it is added as a primary mobile. Numbers not in the local result are kept (no user data in Rex is deleted, only duplicates).
   - Emails: the same, compared in lower case, with `email_primary`.
   - `contact_by_*` from the local consent.
3. Tags: case-insensitive duplicates are removed in a **separate** `Contacts::update` (`contact_tags: [{id, tag, _destroy: true}]`; the 2026-11-17 sandbox run B5 showed Rex rejects a tag removal without the `tag` value). This exact request is not yet re-run in the sandbox: if Rex rejects it, the job still finishes as `sent` with "duplicate tags left in Rex" and a `skipped` row in `rex_sync_events`; the name/phone fix stays.

Each run reads A first, so a retry recomputes from Rex and never appends twice.

**Relation alignment (`rex_merge_relation`, one job per property in the merge result):**
1. `Properties::read`; only rows with contact = A are considered. Rows of any other contact (including B) are never touched.
2. One A row already has the expected type → extra A rows are destroyed, nothing else.
3. A has rows, none of the expected type → the first one is corrected in place: `{id, reln_type: {id: kind}}`. The property is read again; if Rex rejected or ignored the change, that row is destroyed and the expected type is added in one request (`[{id, _destroy: true}, {contact: {id: A}, reln_type: {id: kind}}]`).
4. A has no row → `{contact: {id: A}, reln_type: {id: kind}}` is added.
5. Several A rows → one is kept with the expected type, the others are destroyed.

Every pass re-reads the property, so the job is idempotent; an unknown write outcome is retried instead of failing. Rows without an id cannot be addressed and are reported in the job reason. The earlier rule "no row id found → fail for manual reconcile" is gone.

**Notes:** the merge log note is created on A once (`contact_notes` link). B's notes that were already in Rex move to A with `combineRecords` and are not created again: moving `contact_notes` locally does not queue anything, and a note that is already linked (`rex_links` entity `contact_notes`) is never sent twice. A note of B that was still queued at merge time is sent to the contact its `contact_notes` row now points at (A), not to the trashed B.

**Still to confirm in the sandbox** (`.tmp/contacts/sandbox-merge2.sh` runs the same correction requests and reads back):
- Whether a `contact_names` row update replaces the joined name in every place Rex shows it.
- Whether `Properties::update` accepts an in-place `reln_type` change (the worker falls back to destroy + add either way).
- Whether `contact_tags` accepts `_destroy`.

## Mobile

- The duplicate prompt in Quick add and the "Possible duplicate" card on the contact screen open the merge sheet (`kind: 'merge'`).
- The sheet loads the server preview and shows:
  - which record is kept and why
  - each field and why it won
  - DNC and consent, tags, roles, property relations, the notes and the log note
  - what happens in Rex
  - the warning "This cannot be undone / 合并后无法撤销"
- On a `409`, the sheet shows the fresh preview and the user must confirm again.
- After merging, the app navigates to A.
- Offline, the sheet only explains that merging needs a connection.
- The old field-picker sheet, the local `mergeContacts` and the "submit for admin approval" flow have been removed.
