# Product Requirements Document — Portofolio IHSG & Recommendations

Personal single-user responsive web app to track manual Indonesian Stock Exchange (IHSG) holdings, compute dual-horizon (6–12 month and 3–5 year) Buy/Hold/Sell recommendations through rule-based scoring and local Hermes AI analysis, and recommend pre-filtered liquid buy candidates (LQ45/Kompas100).

## 1. Scope & Boundaries

### Included (v1)
- Single-user local responsive web application.
- **Dockerized**: Multi-stage Dockerfile + `docker-compose.yml` for local execution.
- Portfolio entry: Ticker (IHSG `.JK`), shares (supports **fractional shares** — decimal input), average buy price (**decimal input supported** for averaged positions), buy date.
- Public data collection: Delayed prices (Yahoo Finance `.JK`) and financial statements/company metadata (IDX official endpoints/web data).
- Automated daily background data refresh with toggle, plus on-demand "Refresh now".
- Dual-horizon rule-based recommendation model (Short term: 6–12 months, Long term: 3–5 years).
- Independent Hermes AI analysis triggered on-demand via "Analisis AI" button using local Hermes CLI (`hermes chat -q`).
- Side-by-side dashboard display of Rule-based score and AI assessment, with explicit disagreement highlighting.
- Pre-filtered external stock discovery (LQ45/Kompas100 pre-filter + search).
- **Auto-analysis for non-index tickers**: when a user adds a ticker outside LQ45/Kompas100, the app automatically fetches market data and generates rule-based scoring + Hermes AI analysis for it. No restriction on which IHSG tickers can be tracked.
- **Premium dark-mode UI** (Linear-inspired design system): near-black canvas, semi-transparent surfaces, tight type, tabular monospace numbers, intentional accent color, no AI-slop defaults.
- Indonesian UI language and Indonesian AI explanations.

### Excluded (v1)
- Multi-user authentication/accounts.
- Live real-time intraday trading feeds.
- Fee, dividend, cash balance, and tax tracking.
- Automated email/push alerts.
- External broker auto-sync.

## 2. Core Workflows

### W1: Portfolio Management
1. User adds/edits/deletes held positions (`ticker`, `shares`, `avg_buy_price`, `buy_date`).
2. App calculates total cost, current value, total profit/loss (IDR & %), and overall allocation %.

### W2: Rule-Based Scoring Engine
1. Computes sub-scores for Valuation, Financial Growth, Profitability, Debt/Solvency, and Technical Trend.
2. Short-term (6–12m): Weights Technical trend (moving averages, momentum, volume) + short-term earnings.
3. Long-term (3–5y): Weights fundamental health (ROE, Debt-to-Equity, PE/PBV, revenue growth) + broad trend.
4. Produces `Buy`, `Hold`, or `Sell` verdict per horizon.

### W3: Local Hermes AI Analysis
1. User clicks "Analisis AI".
2. Backend invokes local `hermes chat -q` with context: ticker, portfolio weight, delayed price series, financial ratios, rule score/breakdown, and source links.
3. Hermes returns structured JSON: Short-term verdict + reasoning, Long-term verdict + reasoning, risk factors, and data limitations.
4. App stores AI output and renders side-by-side comparison with rule-based score.

### W4: Pre-filtered External Recommendations
1. App maintains pre-filtered list of liquid IHSG stocks (LQ45/Kompas100).
2. Runs rule-based screening to rank top Buy opportunities.
3. User can search any specific ticker outside pre-filters.

## 3. Data Entities

- **Position**: `id`, `ticker`, `shares` (float64 — supports fractional shares), `avg_buy_price` (float64 — supports decimal precision), `buy_date`, `created_at`, `updated_at`.
- **MarketData**: `ticker`, `company_name`, `last_price`, `pe_ratio`, `pbv_ratio`, `roe`, `der`, `rev_growth`, `net_margin`, `ma20`, `ma50`, `ma200`, `updated_at`.
- **AnalysisResult**: `id`, `ticker`, `horizon` (short/long), `rule_verdict`, `rule_score`, `rule_breakdown_json`, `ai_verdict`, `ai_explanation`, `ai_updated_at`.
- **Settings**: `refresh_enabled`, `last_background_refresh`.

## 4. Non-Functional Requirements

- **Localization**: UI text and generated AI explanations in Indonesian.
- **Reliability**: App functions offline for portfolio viewing; clear visual indicator for delayed/stale data timestamps.
- **Portability**: Single local application process/package.
