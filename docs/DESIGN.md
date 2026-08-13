# DESIGN — Portofolio IHSG & Recommendations

## Design Read
**Reading this as:** Personal financial cockpit for a single Indonesian stock investor. Data-dense but calm. Every number is decision-grade. The UI should feel like a precision instrument — not a generic dashboard template.

**Design direction:** Linear-inspired dark-mode. Near-black canvas, semi-transparent luminance-stacked surfaces, Inter Variable with cv01/ss03 OpenType features, tabular monospace for all financial numbers. Single indigo-violet accent. No gradients, no AI-slop defaults.

## Dials

| Dial | Value | Meaning |
|------|-------|---------|
| DESIGN_VARIANCE | 4/10 | Intentional dark-mode with precision type |
| MOTION_INTENSITY | 2/10 | Fast, functional transitions only |
| VISUAL_DENSITY | 8/10 | Compact tables, dense data, tabular numbers |

## Design System
- **Framework:** Tailwind CSS 4 (CSS custom properties for tokens)
- **Fonts:** Inter Variable (display + body), JetBrains Mono / Berkeley Mono (financial numbers, labels)
- **OpenType:** `font-feature-settings: "cv01", "ss03"` globally on all Inter text
- **Type weights:** 400 (reading), 500 (UI emphasis), 600 (strong emphasis) — never 700+

## Color Palette

### Background Surfaces (luminance stacking)
| Token | Hex | Usage |
|-------|-----|-------|
| `--bg-canvas` | `#08090a` | Page background — deepest canvas |
| `--bg-panel` | `#0f1011` | Sidebar, header, panel backgrounds |
| `--bg-surface` | `#191a1b` | Elevated cards, dropdowns, modals |
| `--bg-hover` | `#28282c` | Hover states, active rows |

### Text
| Token | Hex | Usage |
|-------|-----|-------|
| `--text-primary` | `#f7f8f8` | Headlines, prices, ticker symbols (NOT pure white) |
| `--text-secondary` | `#d0d6e0` | Body text, descriptions |
| `--text-muted` | `#8a8f98` | Labels, timestamps, metadata |
| `--text-faint` | `#62666d` | Disabled, subtle labels |

### Brand Accent (the ONLY chromatic color in UI chrome)
| Token | Hex | Usage |
|-------|-----|-------|
| `--accent` | `#5e6ad2` | Primary buttons, active tabs |
| `--accent-hover` | `#7170ff` | Interactive hover |
| `--accent-light` | `#828fff` | Links, active states |

### Status Colors (used sparingly — verdicts, P&L only)
| Token | Hex | Usage |
|-------|-----|-------|
| `--success` | `#27a644` | Buy verdict, positive P&L |
| `--warning` | `#eab308` | Hold verdict, stale data |
| `--danger` | `#ef4444` | Sell verdict, negative P&L |
| `--danger-bg` | `rgba(239,68,68,0.1)` | Sell verdict badge bg |

### Borders (always semi-transparent white, never solid dark)
| Token | Value | Usage |
|-------|-------|-------|
| `--border-subtle` | `rgba(255,255,255,0.05)` | Default — whisper-thin |
| `--border` | `rgba(255,255,255,0.08)` | Cards, inputs, standard |
| `--border-strong` | `rgba(255,255,255,0.12)` | Hover emphasis |

## Typography Scale

| Role | Font | Size | Weight | Letter Spacing | Notes |
|------|------|------|--------|----------------|-------|
| Display | Inter | 32px | 500 | -0.704px | Page titles |
| Heading | Inter | 24px | 500 | -0.288px | Section titles |
| Subheading | Inter | 20px | 600 | -0.24px | Card headers |
| Body | Inter | 16px | 400 | normal | Standard text |
| Body Small | Inter | 14px | 400 | normal | Labels, descriptions |
| Caption | Inter | 13px | 500 | -0.13px | Metadata, timestamps |
| Label | Inter | 12px | 500 | normal | Buttons, tags |
| Ticker | JetBrains Mono | 14px | 500 | -0.02em | Ticker symbols |
| Number | JetBrains Mono | 14px | 500 | normal | Prices, P&L, ratios (tabular) |
| Number Large | JetBrains Mono | 28px | 500 | -0.02em | KPI values |

## Component Specs

### Cards
- Background: `rgba(255,255,255,0.02)` — always translucent, never solid
- Border: `1px solid rgba(255,255,255,0.08)`
- Radius: 8px (standard), 12px (featured panels)
- Hover: background opacity increases to `rgba(255,255,255,0.04)`

### Buttons
- **Primary**: `bg: #5e6ad2`, `text: #fff`, `radius: 6px`, `padding: 8px 16px`
- **Ghost**: `bg: rgba(255,255,255,0.02)`, `border: 1px solid rgba(255,255,255,0.08)`, `text: #d0d6e0`
- **Danger**: `border: 1px solid rgba(239,68,68,0.3)`, `text: #ef4444`

### Verdict Badges
- **Buy**: `bg: rgba(39,166,68,0.15)`, `text: #27a644`, `border: 1px solid rgba(39,166,68,0.3)`, `radius: 4px`
- **Hold**: `bg: rgba(234,179,8,0.15)`, `text: #eab308`, `border: 1px solid rgba(234,179,8,0.3)`
- **Sell**: `bg: rgba(239,68,68,0.15)`, `text: #ef4444`, `border: 1px solid rgba(239,68,68,0.3)`

### Inputs
- Background: `rgba(255,255,255,0.02)`
- Border: `1px solid rgba(255,255,255,0.08)`
- Focus border: `#5e6ad2`
- Text: `#f7f8f8`, placeholder: `#62666d`
- Radius: 6px

### Tables
- Header: `bg: rgba(255,255,255,0.02)`, `text: #8a8f98`, `font-size: 12px`, `font-weight: 500`, `text-transform: uppercase`, `letter-spacing: 0.05em`
- Row border: `1px solid rgba(255,255,255,0.05)`
- Row hover: `bg: rgba(255,255,255,0.02)`
- Numbers: always `font-family: 'JetBrains Mono'`, `font-variant-numeric: tabular-nums`

### Disagreement Banner
- `bg: rgba(234,179,8,0.08)`, `border: 1px solid rgba(234,179,8,0.2)`, `text: #eab308`
- Indonesian text: "⚠ Beda Pendapat: Aturan {X} vs AI {Y}"

## Key Screens

### 1. Dashboard Portofolio (Utama)
- **Header bar** (sticky, `--bg-panel`): App name left, refresh button + settings icon right
- **KPI Row** (4 translucent cards): Total Investasi, Nilai Saat Ini, Total P&L (IDR + %), Status Data
- **Tab bar**: "Portofolio" | "Peluang" (active = `--accent` underline)
- **Holdings table**: Ticker (mono), Shares, Avg Price, Current Price, P&L (green/red), Short-term verdict badge, Long-term verdict badge, Actions (Detail, AI, Edit, Delete)

### 2. Stock Detail Modal
- **Backdrop**: `rgba(0,0,0,0.85)` overlay
- **Modal** (`--bg-surface`, 12px radius): Company name + ticker (mono), close icon button (top-right)
- **Market data grid**: 2-col layout, all numbers in mono
- **Horizon cards** (stacked): Each shows Rule score bar + breakdown + AI verdict card side-by-side
- **Disagreement banner** if applicable

### 3. Peluang (Opportunities)
- **Filter bar**: Index filter pills (LQ45 / Kompas100 / Both), Buy-only toggle, search input
- **Ranked table**: Same column treatment as holdings, sorted Buy-first

## Anti-Defaults
- ❌ No bright gradient backgrounds
- ❌ No pure white text (#fff) — always #f7f8f8
- ❌ No solid dark backgrounds for buttons/cards — always translucent
- ❌ No non-tabular numbers in financial tables
- ❌ No bold weight (700) — max 600
- ❌ No warm colors in UI chrome — cool gray + indigo only
- ❌ No visible opaque borders on dark surfaces — always semi-transparent white
- ✅ All financial numbers in JetBrains Mono with tabular-nums
- ✅ Ticker symbols in monospace
- ✅ Semi-transparent surfaces for depth (luminance stacking)
- ✅ Clear green/yellow/red for Beli/Tahan/Jual only
