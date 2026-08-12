# Project Brief — Portofolio IHSG & Recommendations

## Overview
A personal, local responsive web tool designed to track manually inputted Indonesian (IHSG) stock investments, evaluate portfolio health, and deliver dual-horizon recommendations (6–12 months and 3–5 years). It pairs deterministic rule-based quantitative scores with second-opinion AI assessments from local Hermes CLI calls, highlighting any strategy disagreements.

## Key Objectives
1. **Manual Tracking**: Clean, simple portfolio input for IHSG stocks.
2. **Dual-Horizon Strategy**: Short-term (6–12 months) and long-term (3–5 years) Buy/Hold/Sell guidance.
3. **Hybrid Engine**: Deterministic rules + local Hermes AI analysis side-by-side.
4. **Opportunity Discovery**: Pre-filtered liquid buy suggestions (LQ45/Kompas100).
5. **Local & Simple**: Single repository, local background refresh, no external user accounts.

## Tech Stack Strategy
- **Architecture**: Single repository (Monolith / unified app with Docker & Docker Compose).
- **Containerization**: Multi-stage Dockerfile (Go + Next.js build) + `docker-compose.yml` for local execution. Host `hermes` binary mounted/accessed via volume or host networking for the AI bridge.
- **Language**: Indonesian UI.
- **Data Collection**: Public delayed web data (Yahoo Finance `.JK` + official IDX data endpoints).
- **AI Integration**: Local Hermes CLI (`hermes chat -q`) invoked via background subprocess.
