# Market data imports

Admin UI or `axleo market-import <file> --provider pricefinder --kind auto [--dry-run]` accepts Pricefinder Address Search XLS (BIFF8), XLSX and CSV (UTF-8/BOM), up to 20 MB. Format is detected by content, not extension; only the first worksheet is read. Dates accept ISO or day/month/year and are stored in ISO format. Mapping aliases can be overridden with `server/market-mappings/pricefinder.json`.

A file with `Current Owners` and `Last Sale Date` but no Kind column is treated as an ownership snapshot. Matched properties receive ownership and latest sale data; dated sales are stored with stable PDS ID/date identity. Agents are notified only when a sale is newer than the previously stored date and within the last 90 Brisbane calendar days. Footer and blank rows count as ignored. Dry-run validates/matches without writing. Duplicate file hashes are rejected on committed imports.

Only non-personal selected columns are kept in market event raw JSON. Owner names are stored solely in property ownership, visible only to covering agents and admins. Owner addresses, vendor and purchaser names are never stored. A boolean absenteeOwner is computed from the owner address and property street, then exposed under `market` only to covering agents and admins. Licensed provider data must not be redistributed; there is no bulk export.

Admin API: `POST /api/v1/admin/market/imports` multipart file, kind, provider and dryRun; `GET /api/v1/admin/market/imports` and `GET /api/v1/admin/market/imports/{id}`. Pricefinder API pulling remains disabled pending an authorized API contract. XLS parsing is manually smoke-tested against a local licensed export; no real export or derived fixture is kept in this repository.
