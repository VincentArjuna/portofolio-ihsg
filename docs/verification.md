# T7 — End-to-End Verification Runbook

Issue #7. Captures what was validated end-to-end and how to re-run it. The
verification lives in `e2e_test.go` as `TestEndToEnd` (10 sub-tests), driven
through the real Fiber app in-process (`app.Test`) with the live Yahoo fetcher
swapped for a deterministic stub — so the full refresh → score → P&L → verdict
pipeline runs offline in ~0.15s.

## How to run

```bash
go test -run TestEndToEnd -v ./...
```

All other checks confirmed clean on the T7 branch:

```bash
go test ./...      # ok  portofolio-ihsg
go build ./...     # OK
go vet ./...       # OK
( cd web && npm run build )   # ✓ Compiled successfully, static export OK
```

## Acceptance criteria → verified

| # | Issue acceptance criterion | Sub-test | What is asserted |
|---|---------------------------|----------|------------------|
| 1 | Sample portfolio (BBCA, BBRI, TLKM, ASII) loads and refreshes end to end | `add_positions`, `market-data_refresh`, `portfolio_P&L_and_verdicts` | 4 positions POSTed (201); `POST /market-data/refresh` returns `refreshed_count ≥ 4`, `failed = 0`; `GET /portfolio` returns 4 rows with `current_price`/`current_value_idr`/`profit_loss_idr` populated. BBCA P&L checked exactly: 1000 × (9750 − 9000) = **+750000**. |
| 2 | Dual-horizon Rule + AI verdicts display with disagreement detection | `portfolio_P&L_and_verdicts`, `stock_detail_breakdown`, `rule-vs-AI_disagreement` | Each position carries `verdicts.short_term.rule` + `long_term.rule` (non-empty) pre-AI; `GET /stocks/BBCA` returns both-horizon `rule.breakdown` whose entries sum exactly to `score`. After a stub Hermes run returning `HOLD`, BBRI shows **Rule BUY vs AI HOLD → `disagreement = true`**. |
| 3 | Hermes-unavailable fallback verified (no crash, clear state) | `hermes_unavailable_fallback` | With `hermes_executable` pointed at a non-existent path, `POST /stocks/BBCA/ai-analyze` returns **HTTP 200** (not 500) with `status = "unavailable"` and no stored AI verdict. |
| 4 | Offline viewing of held positions with stale-data indicators verified | `offline_stale_viewing` | After forcing TLKM's `updated_at` 48h into the past, `GET /stocks/TLKM` still returns full market data (offline-readable) and the old `updated_at` timestamp is surfaced so the frontend `isStale` flag fires. |
| 5 | Background refresh on/off toggle verified against the scheduler | `scheduler_toggle` | Enabled + overdue → `runScheduledRefresh` refreshes (TLKM `updated_at` advances, `last_background_refresh` stamped). Disabled → a subsequent `runScheduledRefresh` leaves data untouched. |
| 6 | Verification notes/runbook captured | this file | — |

## Coverage map (sub-tests)

- `add_positions` — POST `/portfolio`, ticker normalization (lowercase → upper).
- `market-data_refresh` — POST `/market-data/refresh`; `refreshed_count`, `failed`, `updated_at`.
- `portfolio_P&L_and_verdicts` — GET `/portfolio`; summary live fields, per-position P&L, both-horizon rule verdicts, AI nil pre-run.
- `stock_detail_breakdown` — GET `/stocks/BBCA`; both-horizon 5-factor breakdown present and summing to score.
- `opportunities_ranked_universe` — GET `/opportunities` (held excluded, Buy-first ranked), `?filter=lq45` membership filter, `/lookup` illiquid flag.
- `settings_round-trip` — GET/PUT `/settings`; defaults + persistence across reads.
- `hermes_unavailable_fallback` — missing-binary → `unavailable`, no crash, no stored AI.
- `rule-vs-AI_disagreement` — stub `hermes` binary → `status = done`, AI verdict persisted, disagreement rendered, 24h cache → second call `cached`.
- `offline_stale_viewing` — stale data still served + timestamp surfaced.
- `scheduler_toggle` — `runScheduledRefresh` honours the on/off toggle.

## Testability seams introduced

1. `fetchMarketDataFn` (`marketdata.go`) — package-level var defaulting to the
   real Yahoo fetcher; the E2E test overrides it with a deterministic stub so no
   network is touched. No behaviour change in production.
2. `setupApp(db)` (`main.go`) — extracted route wiring from `main()` so the test
   can build the identical app the real server runs, then drive it with
   `app.Test` (no port, in-process).
