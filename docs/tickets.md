# Ticket Plan — Portofolio IHSG & Recommendations

## Phase 1 — Core Infrastructure & Data Engine
- [ ] **T-101**: Initialize Go Fiber monolith repo structure with SQLite & GORM models.
- [ ] **T-102**: Implement Market Data Fetcher module for Yahoo Finance `.JK` delayed quotes & technical indicators (MA20, MA50, MA200).
- [ ] **T-103**: Implement IDX company data parser for financial statement metrics (PER, PBV, ROE, DER, Rev Growth, Net Margin).
- [ ] **T-104**: Implement background cron scheduler for daily refresh with settings toggle.

## Phase 2 — Rule-Based Recommendation Engine
- [ ] **T-201**: Implement Short-Term (6–12m) Rule Engine (Technical 70% + Fundamental 30%).
- [ ] **T-202**: Implement Long-Term (3–5y) Rule Engine (Fundamental 85% + Technical 15%).
- [ ] **T-203**: Add risk profile rules (Balanced-Growth penalties & bonus checks).

## Phase 3 — Local Hermes AI Integration Bridge
- [ ] **T-301**: Build Go subprocess bridge executing `hermes chat -q` with context JSON.
- [ ] **T-302**: Define prompt template demanding structured JSON output in Indonesian.
- [ ] **T-303**: Create async handler for "Analisis AI" button and store output in `AnalysisResult`.

## Phase 4 — Frontend Dashboard & UI
- [ ] **T-401**: Build Responsive Layout & Header with summary KPI cards & Data Refresh status controls.
- [ ] **T-402**: Build Portfolio Holdings Table with inline Rule vs AI verdict badges & "Beda Pendapat" highlighting.
- [ ] **T-403**: Build Add/Edit/Delete Position modals.
- [ ] **T-404**: Build Stock Detail Modal/View displaying side-by-side Rule vs AI analysis breakdowns.
- [ ] **T-405**: Build Opportunities View for pre-filtered LQ45/Kompas100 top buy recommendations & custom ticker lookup.

## Phase 5 — Verification & End-to-End Testing
- [ ] **T-501**: End-to-end verification with sample IHSG portfolio (e.g. BBCA, BBRI, TLKM, ASII).
- [ ] **T-502**: Verify local Hermes CLI execution, response parsing, and fallback handling when Hermes CLI is unavailable.
