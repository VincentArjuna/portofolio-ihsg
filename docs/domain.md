# Domain Model — Portofolio IHSG & Recommendations

## Entities

### Position (Saham yang Dimiliki)
| Field | Type | Description |
|-------|------|-------------|
| id | UUID | PK |
| ticker | string | Kode saham IHSG (e.g., "BBCA") |
| shares | int | Jumlah lot/saham |
| avg_buy_price | decimal | Harga beli rata-rata (IDR) |
| buy_date | date | Tanggal pembelian |
| created_at | timestamp | |
| updated_at | timestamp | |

### MarketData (Data Pasar untuk Ticker)
| Field | Type | Description |
|-------|------|-------------|
| ticker | string | PK, FK ke idx ticker list |
| company_name | string | Nama perusahaan |
| last_price | decimal | Harga terakhir (delayed) |
| prev_close | decimal | Harga tutup sebelumnya |
| pe_ratio | decimal | Price-to-Earnings |
| pbv_ratio | decimal | Price-to-Book Value |
| roe | decimal | Return on Equity (%) |
| der | decimal | Debt-to-Equity Ratio |
| rev_growth | decimal | Pertumbuhan revenue YoY (%) |
| net_margin | decimal | Laba bersih terhadap revenue (%) |
| ma20 | decimal | Moving average 20 hari |
| ma50 | decimal | Moving average 50 hari |
| ma200 | decimal | Moving average 200 hari |
| source_url | string | URL sumber data |
| updated_at | timestamp | Kapan data di-fetch |

### AnalysisResult (Hasil Analisis per Ticker)
| Field | Type | Description |
|-------|------|-------------|
| id | UUID | PK |
| ticker | string | FK |
| horizon | enum | `short` (6-12 bulan) / `long` (3-5 tahun) |
| rule_verdict | enum | `buy` / `hold` / `sell` |
| rule_score | int | Total skor 0-100 |
| rule_breakdown | json | Sub-skor per kategori |
| ai_verdict | enum | `buy` / `hold` / `sell` / `null` |
| ai_explanation | text | Penjelasan AI dalam Bahasa Indonesia |
| ai_confidence | decimal | 0.0–1.0 |
| ai_risk_factors | json[] | Array risiko yang diidentifikasi AI |
| ai_updated_at | timestamp | Null jika belum dianalisis |

### Settings
| Field | Type | Description |
|-------|------|-------------|
| refresh_enabled | bool | Toggle background refresh |
| last_background_refresh | timestamp | Run terakhir |
| hermes_path | string | Path ke binary `hermes` |

## Relationships

```
Position 1—1 MarketData (via ticker)
Position 1—N AnalysisResult (short + long per ticker)
MarketData 1—N AnalysisResult
```

## Rule-Based Scoring Categories

### Short Term (6–12 Bulan) — Max 100
| Component | Weight | Source |
|-----------|--------|--------|
| Trend Teknis (MA20 vs MA50 vs price) | 30 | Yahoo Finance price history |
| Momentum (rate of change, RSI proxy) | 25 | Yahoo Finance price history |
| Volume Trend | 15 | Yahoo Finance price history |
| Valuasi (PER vs sektor, PBV) | 15 | IDX / Yahoo Finance |
| Earnings Momentum (rev_growth, net_margin trend) | 15 | IDX financial reports |

**Verdict**: ≥65 = Buy, 40–64 = Hold, <40 = Sell

### Long Term (3–5 Tahun) — Max 100
| Component | Weight | Source |
|-----------|--------|--------|
| Profitabilitas (ROE, Net Margin) | 30 | IDX financial reports |
| Solvabilitas (DER) | 20 | IDX financial reports |
| Valuasi (PER, PBV) | 20 | Yahoo Finance / IDX |
| Pertumbuhan (Revenue Growth YoY) | 15 | IDX financial reports |
| Trend Teknis (MA200 position) | 15 | Yahoo Finance price history |

**Verdict**: ≥65 = Buy, 40–64 = Hold, <40 = Sell

## Risk Profile: Balanced-Growth
- Moderate tolerance for volatility in exchange for long-term upside.
- Rule scoring penalizes: DER > 2.0, negative net margin, declining revenue trend.
- Bonus weight to consistent ROE > 15% and revenue growth > 10%.

## Risk Flags
| Flag | Trigger |
|------|---------|
| high_debt | DER > 2.0 |
| low_profitability | ROE < 8% |
| overvalued | PER > 25 or PBV > 3.0 |
| downtrend | Price < MA50 < MA200 |
| illiquid | Not in LQ45/Kompas100 (for external recs) |

## Data Limitations
- Prices are delayed (typically 15+ minutes via Yahoo Finance).
- Financial statements are quarterly/annual, may lag 1–3 months.
- Moving averages require sufficient price history (200 trading days for MA200).
- Not real-time. Not investment advice.
