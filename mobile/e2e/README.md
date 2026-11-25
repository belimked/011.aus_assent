# Mobile E2E (Maestro, iOS simulator)

Maestro flows for the Expo app, run in GitHub Actions on an iOS simulator against a fresh,
seeded copy of the Go server. Workflow: `.github/workflows/mobile-e2e.yml` (**Mobile iOS E2E**).
It only runs when started by hand (`workflow_dispatch`). Pushes and PRs never trigger it.

What can't be automated (shake, real microphone/STT, camera, GPS) is in [MANUAL.md](MANUAL.md).

## Run it

**GitHub UI:** Actions → *Mobile iOS E2E* → *Run workflow*, pick the branch, then:

| Input | Default | Meaning |
| --- | --- | --- |
| `ios` | `26.0` | Simulator runtime (downloaded if the runner lacks it) |
| `flows` | empty = all | Filename glob inside `mobile/e2e/`, e.g. `05-*`, `*offline*`, `0[1-4]-*`; `.yaml` is appended if missing |
| `rebuild` | `false` | Ignore the cached app and build again |

**CLI:**

```sh
gh workflow run mobile-e2e.yml --ref feat/mobile-e2e                       # all flows
gh workflow run mobile-e2e.yml --ref feat/mobile-e2e -f flows='05-*'       # one flow
gh workflow run mobile-e2e.yml --ref feat/mobile-e2e -f ios=18.6 -f rebuild=true
gh run watch; gh run download <run-id>                                     # follow, fetch evidence
```

**Evidence** (artifact `mobile-e2e-ios-<ios>-<attempt>`, uploaded even on failure):
`junit.xml`, `maestro.log`, `debug/` (per-command screenshots and Maestro logs for failures),
`screenshots/` (`takeScreenshot`), `session.mp4` (whole-run recording), `device.log`,
`server.log`, `proxy.log`, and any crash reports.

## What runs where

```
Maestro ──runScript──────────────────────────────► Go server 127.0.0.1:8082 (fresh DB, `axleo seed --with-demo`)
App (EXPO_PUBLIC_API_URL=127.0.0.1:8092/api/v1) ─► CI proxy 127.0.0.1:8092 ─┘
```

- The server is built from `server/`, with a new `AXLEO_DB` in `$RUNNER_TEMP` and
  `AXLEO_SEED_PASSWORD` set to a CI-only value. `axleo seed --with-demo` runs before `axleo serve`. Seeded users are
  `admin@axleo.local`, `mary@axleo.local` and `john@axleo.local`. Flows sign in as Mary.
- `common/network-proxy.cjs` passes everything through to the server. A `POST /__e2e/offline`
  makes it answer 503 to every request until `POST /__e2e/online`. That lets flow 11 simulate lost
  signal, since the iOS simulator has no airplane mode. It lives only in CI. Neither the app nor
  the server changes.
- **The API URL is build-time.** `src/store/sync.ts` reads `process.env.EXPO_PUBLIC_API_URL`, then
  `expo.extra.apiUrl`, then the production URL. Expo inlines the env var into the JS bundle when
  the app is built. Release/TestFlight builds don't set it, so production behaviour is unchanged.
  Pointing the E2E app at another server means rebuilding it. The URL is in the cache key, so
  that happens automatically.

## Simulator app cache

Building takes most of a run: `npm ci`, `expo prebuild`, CocoaPods and a Release `xcodebuild`.
The unsigned simulator `.app` is cached with `actions/cache`. The key is computed in a step,
because `hashFiles()` can't exclude paths:

```
e2e-sim-app-v1-<Xcode version>-<sha of tracked mobile/** except mobile/e2e/** and mobile/docs/**>-<package-lock sha>-<API URL sha>
```

- **Hit** (newest entry for that key, matched by prefix): `npm ci`, prebuild and `xcodebuild` are
  skipped and the cached `.app` is installed. **Editing only flows or docs reuses the cache.**
- **Miss** (app code, assets, `app.json`, `package-lock.json`, Xcode or API URL changed): build, then
  save under `<key>-<run id>`. A cache is saved only if the build succeeded.
- `rebuild: true` skips the restore and saves a fresh entry, which then wins on later restores.
- Changing *how* the app is built (the xcodebuild flags in the workflow) doesn't change the key.
  Bump `v1` in the key when you do that.
- Only git-tracked files are hashed, so commit before running.

Typical time on `macos-26`: about 10 min on a hit (server build, simulator boot, Maestro install,
12 flows) and about 30–40 min on a miss.

## Flows

| # | File | Covers |
| --- | --- | --- |
| 1 | `01-login-logout.yaml` | sign in, sign out clears token and cached data (relaunch stays on login), bad password, sign in again |
| 2 | `02-dk.yaml` | door knock: type, "Check before saving", approve → "Saved", note on the record |
| 3 | `03-contacts.yaml` | add an owner, opt out of SMS, tag Hot |
| 4 | `04-todos.yaml` | create, done, undo, reassign to John |
| 5 | `05-fullmap.yaml` | full-screen map: preview card over the map (screenshot), note, open record, close card |
| 6 | `06-translate.yaml` | Translate opened via its button on Me, switch visitor to Cantonese |
| 7 | `07-ofi.yaml` | OFI check-in of a new visitor |
| 8 | `08-reviews.yaml` | review queue: approve one capture, swipe-dismiss another |
| 9 | `09-admin-to-phone.yaml` | `runScript` creates a to-do via the admin API → it appears on the phone |
| 10 | `10-phone-to-admin.yaml` | note on the phone → `runScript` finds it via the admin API |
| 11 | `11-offline.yaml` | offline save → "1 waiting to sync" → network back → Retry now → synced, on the server |
| 12 | `12-language.yaml` | English ↔ 中文 on Me |

Each flow starts with `common/reset.yaml`: clear keychain and app data, launch with permissions
granted, sign in as Mary, and wait for the first sync. Flows don't depend on each other, but they
share one server DB, so they never assume a pristine server. Use unique `E2E …` text.

## Adding a flow

1. Create `mobile/e2e/NN-name.yaml` with `appId: com.placesunnybank.agentcollect`, a `name:`,
   `---`, then `- runFlow: common/reset.yaml`. Top-level `*.yaml` files run in name order. Shared
   pieces live in `common/` and only run through `runFlow` or `runScript`.
2. Prefer `id:` selectors (`testID`). Add a `testID` in `mobile/src` when an element has none. It
   must be additive with no visual change. `Button`, `Card`, `Chip`, `Chips` (`<id>-<value>`), `ListItem`
   and `Logo` accept `testID`, and `Field` passes it to its `TextInput`. Note that a source change
   invalidates the app cache, so the next run rebuilds.
3. Text selectors are **full-match, case-insensitive regexes**. Escape `+`, `?`, `(`, and use
   `.*` for partial text. On iOS a pressable with several texts (list rows, tiles) is one element
   whose label joins its texts, so use a `testID` or `'Title.*'`. Toasts are plain text (`Saved`).
4. Server-side setup or checks go in `runScript` JS (GraalJS: `http.get/post`, `json`, no
   `require`). `API`, `PROXY` and `E2E_PASSWORD` come from `maestro -e`. See
   `common/admin-create-todo.js`.
5. Locally, on a Mac with a booted simulator that has a build installed (built with
   `EXPO_PUBLIC_API_URL=http://127.0.0.1:8092/api/v1`), run:

   ```sh
   (cd server && go build -o /tmp/axleo . && AXLEO_DB=/tmp/e2e.db AXLEO_SEED_PASSWORD=dev-e2e-pass /tmp/axleo seed --with-demo \
     && AXLEO_DB=/tmp/e2e.db AXLEO_ADDR=127.0.0.1:8082 /tmp/axleo serve &)
   node mobile/e2e/common/network-proxy.cjs &
   maestro test -e E2E_PASSWORD=dev-e2e-pass -e API=http://127.0.0.1:8082/api/v1 -e PROXY=http://127.0.0.1:8092 mobile/e2e/05-fullmap.yaml
   ```

## Why not reuse the Ad Hoc `.ipa`?

`mobile-ios-adhoc.yml` produces a **device** build: arm64 `iphoneos` binaries, signed with the
distribution certificate and an Ad Hoc profile listing specific UDIDs. A simulator can't run it,
because the slices are for the device platform, not `iphonesimulator`. Simulators can't satisfy an
Ad Hoc provisioning profile either. That build also inlines the production API URL. The E2E job
therefore builds its own unsigned simulator `.app`, and the cache keeps that cheap.
