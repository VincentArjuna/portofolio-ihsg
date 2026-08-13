# Portofolio IHSG & Recommendations

Personal single-user local web app to track manual Indonesian Stock Exchange
(IHSG) holdings and produce dual-horizon **Buy / Hold / Sell** recommendations:
a deterministic **rule-based score** alongside an on-demand **local Hermes AI**
analysis, plus a pre-filtered liquid-stock discovery screen (LQ45 / Kompas100).

Single Go process (Fiber + pure-Go SQLite) serving a Next.js static frontend.
Indonesian UI. Delayed data. **Bukan saran investasi** — not investment advice.

## Feature summary

| Area | What it does |
|------|--------------|
| **Portfolio (T1)** | Add/edit/delete positions (`ticker`, `shares`, `avg_buy_price`, `buy_date`); cost basis, current value, P&L (IDR + %), allocation %. |
| **Market data (T2)** | Fetches delayed prices + MAs (Yahoo Finance chart) and best-effort fundamentals (quoteSummary). Manual refresh + daily background scheduler with toggle. |
| **Scoring engine (T3)** | Deterministic 0–100 score per horizon. **Short (6–12m):** Trend Teknis, Momentum, Volume, Valuasi, Earnings. **Long (3–5y):** Profitabilitas, Solvabilitas, Valuasi, Pertumbuhan, Trend. ≥65 Buy, 40–64 Hold, <40 Sell. Risk flags: high debt, low profitability, overvalued, downtrend. |
| **Hermes AI (T4)** | `POST /stocks/:ticker/ai-analyze` runs the local `hermes chat -q` CLI with a structured context snapshot, parses the JSON verdict, and renders **Rule vs AI side-by-side** with explicit disagreement banners. Cached 24h. Degrades to `unavailable`/`error` without crashing. |
| **Opportunities (T5)** | LQ45/Kompas100 universe, rule-scored and ranked Buy-first. Custom-ticker lookup flags out-of-universe as illiquid. |
| **Settings (T6)** | Background refresh toggle + interval + Hermes path. Scheduler re-reads settings live. |
| **E2E verification (T7)** | `e2e_test.go` drives the whole stack in-process; see [`docs/verification.md`](docs/verification.md). |

## Stack

- **Backend:** Go 1.26, [Fiber](https://gofiber.io) v2, [GORM](https://gorm.io) +
  pure-Go SQLite ([glebarez/sqlite](https://github.com/glebarez/sqlite), **no CGO**).
- **Frontend:** Next.js 15 (static export) + React 19 + Tailwind CSS 4.
- **AI:** local `hermes` CLI via `os/exec` (no network calls of its own).

## Prerequisites

- Go ≥ 1.26
- Node.js ≥ 20 (for the frontend build)
- Optional: a `hermes` binary on `PATH` for AI analysis (works without it —
  the UI shows a clear "unavailable" state).

## Run locally

### 1. Backend (API only)

```bash
go run .
# → http://localhost:8080  (API at /api/v1/*)
```

Environment variables (all optional):

| Var | Default | Meaning |
|-----|---------|---------|
| `PORT` | `8080` | Listen port. |
| `DB_PATH` | `portofolio.db` | SQLite file path. |
| `WEB_DIR` | `./web/out` | Next.js static export dir; served at `/` if present. |

### 2. Frontend (full app)

```bash
cd web
npm install
npm run build      # static export → web/out
cd ..
go run .           # serves API + UI on :8080
```

For frontend hot-reload during development, run `npm run dev` in `web/`
(Next dev server) and the Go API separately; point the API client at the Go
server if needed.

## API quick reference

Base URL: `http://localhost:8080/api/v1` — full shapes in [`docs/api-contract.md`](docs/api-contract.md).

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/portfolio` | Summary + positions with P&L and both-horizon verdicts. |
| `POST` | `/portfolio` | Add a position. |
| `PUT` | `/portfolio/:id` | Partial update a position. |
| `DELETE` | `/portfolio/:id` | Delete a position. |
| `POST` | `/market-data/refresh` | Fetch + score held + universe tickers. |
| `GET` | `/stocks/:ticker` | Market data + short/long rule+AI breakdown. |
| `POST` | `/stocks/:ticker/ai-analyze` | Run Hermes AI (returns updated detail). |
| `GET` | `/opportunities` | Ranked non-held LQ45/Kompas100 candidates (`?filter=&min_verdict=&q=`). |
| `GET` | `/opportunities/lookup?q=` | Custom-ticker universe membership. |
| `POST` | `/opportunities/refresh` | Re-fetch + re-score the universe. |
| `GET` `/PUT` | `/settings` | Background refresh toggle, interval, Hermes path. |

## Tests

```bash
go test ./...                       # all unit + integration tests
go test -run TestEndToEnd -v ./...  # the T7 end-to-end suite (10 sub-tests)
go vet ./...                        # static checks
```

The E2E test swaps the live Yahoo fetcher for a deterministic stub
(`fetchMarketDataFn` seam in `marketdata.go`), so it runs fully offline in
~0.15s. The Hermes AI path is exercised through a tiny stub `hermes` shell
script written to a temp dir.

## Docker

Multi-stage build → single Alpine image running the Go binary (which also
serves the built frontend). **No CGO**, so the image works on any Linux.

```bash
docker compose up --build      # http://localhost:8080
```

- `Dockerfile`: stage 1 builds the Next.js export, stage 2 builds the Go binary,
  stage 3 is the minimal runtime.
- `docker-compose.yml`: maps `:8080` and persists SQLite under `./data/` on the
  host. To enable AI inside the container, mount the host `hermes` binary:
  ```yaml
  volumes:
    - /usr/local/bin/hermes:/usr/local/bin/hermes:ro
  ```

## Project structure

```
main.go              initDB, setupApp (routes), main
portfolio.go         Position model + portfolio handlers
marketdata.go        MarketData model, Yahoo fetcher, refresh core, fetcher seam
scoring.go           AnalysisResult model, rule scoring, risk flags, stock detail
ai.go                Hermes CLI bridge (context, subprocess, parse, persist)
opportunities.go     LQ45/Kompas100 universe, filtering, ranking
scheduler.go         background refresh loop (settings-driven)
settings.go          AppSettings model + handlers
e2e_test.go          T7 end-to-end suite
web/                 Next.js frontend (static export)
docs/                PRD, API contract, architecture, domain model, verification
```

## Data sources & limitations

- Prices/fundamentals are **delayed** (Yahoo Finance) and financial statements
  lag 1–3 months. The UI surfaces `updated_at` / `last_market_update` and marks
  data older than 24h as stale (frontend `isStale`).
- Moving averages need sufficient history (200 sessions for MA200).
- Not real-time. **Bukan saran investasi** — not investment advice.
