# Web console E2E

From the repository root, with Node 22+, Go (from `server/go.mod`), and curl installed:

```sh
npm ci --prefix web
npm ci --prefix web/e2e --include=dev
(cd web/e2e && npx playwright install chromium)
cd web/e2e && npm test
```

Playwright starts its own Go server at `127.0.0.1:8083`, builds the Vue assets and Go binary, seeds a **new** database under the repository's ignored `.tmp/web-e2e/`, then runs `axleo seed --with-demo` for cadastral parcels, the Natalie/Owen listings and OFI fixtures, and admin/Mary/John accounts before bootstrapping a leader (Mary) and another team (John) through the API. Production `axleo seed` does not insert demo business records. Stop any other server listening on 8083 first. The server is stopped and its database removed after the run. Set `TMPDIR` to the repo's `.tmp` if using other temporary-file tools; the server script sets it itself. Seed credentials are confined to the test harness.

Use `npm test -- --headed` to watch, or `npm test -- tests/review.spec.js` for a single spec. Failure screenshots, traces and videos live in `web/e2e/test-results/`; the HTML report is in `web/e2e/playwright-report/` (both ignored). CI uploads both even on failure.

## Add a test

Add `tests/<feature>.spec.js` and import `login`, `agentToken`, and `api` from `tests/helpers.js` as needed. Each file must work alone against a new seed. Create uniquely named test records through the API rather than assuming another spec has run; use role and visible-text locators for UI interactions. Avoid fixed sleeps; use Playwright assertions to wait for state. If a stable selector is unavoidable, add a narrowly scoped `data-testid` to the Vue view and rebuild/commit `server/web/` too. The suite runs serially because tests share a database; no spec may depend on execution order.
