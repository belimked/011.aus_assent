# Mobile store performance (40,000 synthetic properties)

Run: `npx jest --runInBand src/store/perf.test.ts` (informational benchmark; no timing assertion). Node on the development machine, one run:

| Operation | Before | After |
| --- | ---: | ---: |
| Single-property update | `structuredClone` 235.2 ms + `JSON.stringify` 11.3 ms | Immer COW `update` 0.4 ms; serialization deferred 800 ms |

The previous production-sized 40,398-property sample measured 172 ms clone + 46 ms stringify on Node. The synthetic benchmark excludes sync diff/enqueue, notifications, rendering, and the eventual AsyncStorage write; it is **not** an iPhone/Hermes latency measurement. Persisted data is still the same full JSON snapshot, now coalesced and flushed on background/inactive. Initial full sync and full-state hydration still process all properties. A property update with online sync still scans the changed collection to compute insertions/deletions; it no longer serializes every unchanged record. Pages that still subscribe to the entire state may re-render on updates; the hot tabs and map/property pages use keyed subscriptions.
