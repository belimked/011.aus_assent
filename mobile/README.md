# AxleoCollect mobile

Expo SDK 58 / Expo Router client. Contacts, registered properties and to-dos sync with the Axleo API after email/password sign-in. Captures, enquiries, listings, market and Rex activity remain local demo data; the Microsoft sign-in is an offline-only demo and never calls the API. The API URL comes from `app.json` (`expo.extra.apiUrl`) or `EXPO_PUBLIC_API_URL` at export/build time. The sample state is generated from `prototype/index.html` by `node tools/extract-i18n.mjs` (run from the repository root). It is stored under AsyncStorage key `ax_state` (version 8); language is stored under `ax_lang`. Offline demo can be reset from Me. Server sign-out clears its token, sync cursor, pending operations and the three cached resource arrays. Tokens use SecureStore on devices and localStorage on Web; the cursor and outgoing queue use AsyncStorage.

- `src/app/`: Expo Router routes and custom four-tab navigation. Add one route file per new screen.
- `src/screens/`: one component per screen; use `Screen` for header, safe area, scrolling and watermark.
- `src/sheets/`: add a component and register its kind in `registry.ts`; call `openSheet({kind, ...params})` from anywhere.
- `src/store/`, `src/domain/`, `src/data/`: state, pure helpers and mock seed; `src/theme/`, `src/components/ui/`, `src/i18n/`: shared presentation.
- Maps and patches only show synced properties (`s.properties`, Rex/Pricefinder); the app bundles no QLD address points.

Run `npx expo start`, `npm run web:export` for local QA, `npm test`, `npx tsc --noEmit` and `npx expo lint`. Signed iOS Ad Hoc builds run through GitHub Actions (`.github/workflows/mobile-ios-adhoc.yml`), not locally.
