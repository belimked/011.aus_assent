# agentCollect · Place Sunnybank

Expo SDK 58 / React Native 0.88 / React 19.3 skeleton, following the AusFlow team's `src/app` Expo Router, theme, Jest and lint conventions. iOS bundle ID / Android package `com.placesunnybank.agentcollect` **are placeholders**; confirm with Place before signing. The icon and splash image are temporary "aC" placeholders; replace with Place-approved artwork before release. Splash background is Place blue `#0b5d8a`. System font for now; TODO: bundle approved Montserrat via expo-font.

```sh
npm install
npm run typecheck && npm run lint && npm test
npx expo start                # development build required (expo-maps)
npx expo run:ios              # macOS / Xcode required
```

## Data and limitations

`node ../tools/build-sunnybank-mobile.mjs` regenerates `src/data/sunnybank.json` from the **JSON assignment only** in `../prototype/sunnybank-data.js`, without executing it. Addresses, parcel rings and areas are real QLD Spatial data (micro-degree deltas from `[153,-27.5]`). **Addresses & parcels © State of Queensland (CC BY 4.0)**. Statuses, owner names, tenure and phone display are deterministic mock data, not Rex CRM records. Multiple units of the same address/lot-plan share one marker with a unit count; individual unit details are not yet shown. Location defaults to 11 Gager St when permission is denied or the device is outside the Sunnybank data area. No audio is recorded; microphone permission is reserved for voice capture. `expo-secure-store` is installed for future token storage; no credentials are stored yet.

## Native map validation

SDK 58 `expo-maps` exposes polygon arrays, polygon click handlers, camera viewport deltas, and map-type toggles on both AppleMaps (iOS MapKit) and GoogleMaps (Android). This implementation renders culled parcel polygons at zoom >=17 (max 600) and property markers at zoom >=15 (max 400); Android uses coloured circles under default markers because GoogleMaps markers do not expose tintColor, while iOS uses coloured marker tints. Below zoom 15 the map displays a zoom hint. Apple marker/polygon click events require **iOS 18+** according to the installed Expo type definitions. Android Google Maps needs a Google Maps API key in the native build config to render tiles; obtain one before device testing. Native rendering, hit testing, map performance and satellite mode still require testing on real devices; JS bundle export does not validate native rendering. Large parcel lists can still strain MapKit/Google Maps during pans; consider a spatial index, lower caps and native profiling. Native maps do not run in web / standard Expo Go.

## Test builds (iOS Ad Hoc)

[English · 中文 setup and installation manual](docs/ios-adhoc-manual.md). The **Mobile iOS Ad Hoc** workflow runs preflight tests on relevant pushes and manual dispatch; with signing secrets configured, it archives an IPA and publishes the install page at <https://belimked.github.io/011.aus_assent/>. Admin must create the new App ID/profile, enable GitHub Pages (GitHub Actions source), and set the three signing secrets. This is a public repo: the IPA is publicly downloadable but installable only on registered devices. On iOS, tapping property markers/parcels needs **iOS 18+**. Android Google Maps needs a Google Maps API key before device testing.

## Next

Rex OAuth via backend (never embed client secret), offline SQLite queue and sync, voice recording/transcription and photo capture, share extension, Android `FLAG_SECURE`, real branding/icons and Montserrat font, device map QA, status legend, privacy review and i18n coverage for static/native strings. iOS Ad Hoc test builds: see "Test builds (iOS Ad Hoc)" below — signing secrets are set in GitHub, never in this repo.
