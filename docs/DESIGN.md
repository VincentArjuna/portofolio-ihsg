# DESIGN — Portofolio IHSG & Recommendations

## Design Read
**Reading this as:** Personal financial dashboard for a single user, with a clean, functional, data-dense language focused on clarity, quick decision-making, and explicit comparison between rule-based and AI recommendations.

## Dials

| Dial | Value | Meaning |
|------|-------|---------|
| DESIGN_VARIANCE | 3/10 | Clean, structured grid layouts |
| MOTION_INTENSITY | 2/10 | Subtle transitions only; fast data presentation |
| VISUAL_DENSITY | 8/10 | Compact tables, status badges, tabular numbers |

## Design System
- **System:** Tailwind CSS + Radix/Base UI components (dense financial dashboard variant).
- **Theme:** Clean dark mode by default (easier on eyes for financial monitoring).

## Color Palette

| Token | Hex | Usage |
|-------|-----|-------|
| bg-primary | #0F172A | Main background (slate-900) |
| bg-surface | #1E293B | Cards, table headers, containers (slate-800) |
| text-primary | #F8FAFC | Headlines, prices, ticker symbols |
| text-muted | #94A3B8 | Labels, timestamps, secondary details |
| accent | #38BDF8 | Primary action buttons, active tab (sky-400) |
| success | #22C55E | Buy verdict, positive P&L (green-500) |
| warning | #EAB308 | Hold verdict, neutral indicators (yellow-500) |
| danger | #EF4444 | Sell verdict, negative P&L (red-500) |
| border | #334155 | Dividers, card borders (slate-700) |

## Typography
- **Display / Headings**: Geist / Inter (bold, crisp uppercase tickers)
- **Body / Labels**: Inter / System UI
- **Data / Numbers**: Tabular monospace for prices, percentages, ratios, and dates.

## Key Screens Layout

### 1. Dashboard Portofolio (Utama)
- Top bar: Summary metrics (Total Nilai Portofolio, Total Gain/Loss IDR & %, Status Refresh & Toggle, Tombol Refresh Manual).
- Main area: Table of held stocks with columns: Ticker, Shares, Avg Price, Current Price, Gain/Loss, Short-Term Verdict (Rule vs AI), Long-Term Verdict (Rule vs AI), Action ("Analisis AI").
- Disagreement banner/badge highlighted when Rule Verdict != AI Verdict.

### 2. Detail Saham & Analisis Side-by-Side
- Left column: Financial metrics & technical indicators (PER, PBV, ROE, DER, MA20/50/200).
- Right top: Short-Term (6–12 Bulan) comparison card (Rule score breakdown vs AI explanation).
- Right bottom: Long-Term (3–5 Tahun) comparison card (Rule score breakdown vs AI explanation).

### 3. Rekomendasi Saham Eksternal (LQ45 / Kompas100)
- Pre-filtered list of top Buy recommendations for non-held stocks.
- Search input for custom ticker lookup.

## Signature Element
Side-by-side **Aturan vs AI** verdict cards with explicit disagreement indicators (`Beda Pendapat: Aturan Beli vs AI Tahan`).

## Anti-Defaults
- ❌ No bright decorative gradient backgrounds.
- ❌ No floating action buttons or marketing banners.
- ❌ No non-tabular numbers in financial tables.
- ✅ Clear green/yellow/red status indicators for Beli/Tahan/Jual.
- ✅ Explicit data timestamps on every card.

## Component States
- **Loading**: Skeleton rows matching table column shapes.
- **Empty**: "Belum ada saham di portofolio. Tambah saham pertama Anda."
- **AI Generating**: Spinner on button + "Hermes sedang menganalisis data..." state.

## Shape Language
- Cards: `rounded-lg` (8px)
- Buttons & Badges: `rounded-md` (6px)
- Table container: `rounded-lg border border-slate-700`
