# AxleoCollect mobile

Expo SDK 58 / Expo Router client. **All records and Rex activity are mock data**; no backend or Rex API calls are made. The sample state is generated from `prototype/index.html` by `node tools/extract-i18n.mjs` (run from the repository root). It is stored under AsyncStorage key `ax_state` (version 8); language is stored under `ax_lang`. Reset from Me.

- `src/app/`: Expo Router routes and custom four-tab navigation. Add one route file per new screen.
- `src/screens/`: one component per screen; use `Screen` for header, safe area, scrolling and watermark.
- `src/sheets/`: add a component and register its kind in `registry.ts`; call `openSheet({kind, ...params})` from anywhere.
- `src/store/`, `src/domain/`, `src/data/`: state, pure helpers and mock seed; `src/theme/`, `src/components/ui/`, `src/i18n/`: shared presentation.
- Existing QLD parcel geometry is in `src/data/sunnybank.json`, `src/data/properties.ts` and `src/lib/geometry.ts` for the map phase.

Run `npx expo start`, `npm run web:export` for local QA, `npm test`, `npx tsc --noEmit` and `npx expo lint`. Signed iOS Ad Hoc builds run through GitHub Actions (`.github/workflows/mobile-ios-adhoc.yml`), not locally.
