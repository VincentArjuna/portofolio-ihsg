# API Contract — Portofolio IHSG & Recommendations

Base URL: `http://localhost:8080/api/v1`

## Portfolio Endpoints

### 1. List Portfolio
`GET /portfolio`

**Response 200 OK:**
```json
{
  "summary": {
    "total_investment_idr": 50000000,
    "current_value_idr": 54200000,
    "total_profit_loss_idr": 4200000,
    "total_profit_loss_pct": 8.4,
    "last_market_update": "2026-08-11T09:30:00Z"
  },
  "positions": [
    {
      "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "ticker": "BBCA",
      "company_name": "PT Bank Central Asia Tbk",
      "shares": 1000,
      "avg_buy_price": 6000,
      "buy_date": "2025-01-15",
      "current_price": 6200,
      "current_value_idr": 6200000,
      "profit_loss_idr": 200000,
      "profit_loss_pct": 3.33,
      "weight_pct": 11.44,
      "verdicts": {
        "short_term": {
          "rule": "BUY",
          "ai": "HOLD",
          "disagreement": true
        },
        "long_term": {
          "rule": "BUY",
          "ai": "BUY",
          "disagreement": false
        }
      }
    }
  ]
}
```

### 2. Add Position
`POST /portfolio`

**Request Body:**
```json
{
  "ticker": "BBRI",
  "shares": 500,
  "avg_buy_price": 3100,
  "buy_date": "2026-02-10"
}
```

**Response 201 Created:**
```json
{
  "id": "e31bc10b-12cc-4372-a567-0e02b2c3d480",
  "ticker": "BBRI",
  "shares": 500,
  "avg_buy_price": 3100,
  "buy_date": "2026-02-10"
}
```

### 3. Update Position
`PUT /portfolio/:id`

**Request Body:**
```json
{
  "shares": 600,
  "avg_buy_price": 3080
}
```

### 4. Delete Position
`DELETE /portfolio/:id`

**Response 204 No Content**

---

## Analysis Endpoints

### 5. Get Stock Detail & Analysis
`GET /stocks/:ticker`

**Response 200 OK:**
```json
{
  "ticker": "BBCA",
  "company_name": "PT Bank Central Asia Tbk",
  "market_data": {
    "last_price": 6200,
    "prev_close": 6375,
    "pe_ratio": 18.5,
    "pbv_ratio": 3.2,
    "roe": 18.2,
    "der": 0.2,
    "rev_growth": 12.4,
    "net_margin": 24.1,
    "ma20": 6150,
    "ma50": 6000,
    "ma200": 5800,
    "updated_at": "2026-08-11T09:30:00Z"
  },
  "short_term": {
    "horizon": "6-12 Bulan",
    "rule": {
      "verdict": "BUY",
      "score": 78,
      "breakdown": {
        "trend_tekis": 25,
        "momentum": 20,
        "volume": 12,
        "valuasi": 11,
        "earnings_growth": 10
      }
    },
    "ai": {
      "verdict": "HOLD",
      "explanation": "Meskipun tren teknis jangka pendek masih positif (harga di atas MA20 & MA50), valuasi PER 18.5x sudah mendekati batas atas area wajar sektor perbankan...",
      "confidence": 0.82,
      "risk_factors": ["Take profit jangka pendek di area resistance 6400", "Potensi koreksi IHSG secara umum"],
      "updated_at": "2026-08-11T09:35:00Z"
    },
    "disagreement": true
  },
  "long_term": {
    "horizon": "3-5 Tahun",
    "rule": {
      "verdict": "BUY",
      "score": 85,
      "breakdown": {
        "profitabilitas": 28,
        "solvabilitas": 20,
        "valuasi": 12,
        "pertumbuhan": 12,
        "trend_jangka_panjang": 13
      }
    },
    "ai": {
      "verdict": "BUY",
      "explanation": "Fundamental BBCA sangat solid dengan ROE 18.2% dan DER 0.2x yang sangat konservatif...",
      "confidence": 0.91,
      "risk_factors": ["Perubahan regulasi perbankan"],
      "updated_at": "2026-08-11T09:35:00Z"
    },
    "disagreement": false
  }
}
```

### 6. Trigger Local Hermes AI Analysis
`POST /stocks/:ticker/ai-analyze`

**Response 202 Accepted:**
```json
{
  "status": "processing",
  "message": "Analisis Hermes AI sedang berjalan untuk ticker BBCA..."
}
```

---

## Market Data & Discovery Endpoints

### 7. Trigger Manual Refresh Data
`POST /market-data/refresh`

**Response 200 OK:**
```json
{
  "refreshed_count": 12,
  "updated_at": "2026-08-11T09:40:00Z"
}
```

### 8. List External Opportunities (LQ45 / Kompas100 Filter)
`GET /opportunities?filter=lq45&min_verdict=BUY`

**Response 200 OK:**
```json
{
  "opportunities": [
    {
      "ticker": "TLKM",
      "company_name": "PT Telkom Indonesia Tbk",
      "last_price": 2600,
      "short_term_rule": "BUY",
      "short_term_score": 81,
      "long_term_rule": "BUY",
      "long_term_score": 88,
      "roe": 16.5,
      "per": 11.2
    }
  ]
}
```

---

## Settings Endpoints

### 9. Get & Update Settings
`GET /settings`
`PUT /settings`

**Request / Response Body:**
```json
{
  "background_refresh_enabled": true,
  "refresh_interval_hours": 24,
  "hermes_executable": "hermes"
}
```
