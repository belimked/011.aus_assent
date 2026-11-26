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

## Rex

The local merge commits first. Rex work runs afterwards through the outbox (`rex_merge`) and follows the normal authorisation and retry rules.

| Rex state at merge | What happens |
|---|---|
| Both linked | `Dedupe::combineRecords` (`winning_id` = A, `losing_ids` = [B]). Then A is updated (names, phone, email, `contact_by_*`), DNC is set if needed, relations are fixed, and the log note is created. |
| Only B linked | B's `rex_links` row is moved to A, then A is updated as above. |
| Only A linked | A is updated only. |
| Neither linked | No Rex job. |
| A create is still queued | The job waits for the link, for at most 10 minutes (`mergeLinkWait`). After that it is skipped with "timed out waiting for the Rex contact link". Legacy jobs without a deadline use the outbox `created_at`. |

**Relation fix (`rex_merge_relation`, one job per changed property):**
1. `Properties::read`.
2. `Properties::update`. Rows of A or B whose relation type was superseded are sent as `{id, _destroy: true}`. The winning relation is added for A if Rex does not already have it.

If a row id cannot be found, the job fails with a reconcile reason instead of guessing.

**Needs sandbox verification:**
- The exact parameters `combineRecords` expects.
- That `_destroy` removes `contact_reln_property` rows.
- That relation rows of the losing contact are reassigned to the winner after combining.

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
