package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// e2e_test.go — T7 end-to-end verification.
//
// Drives the whole stack through the real Fiber app (setupApp) over in-process
// HTTP (app.Test): add positions → refresh market data → portfolio P&L +
// verdicts → stock-detail breakdown → LQ45/Kompas100 opportunities → settings
// toggle → Hermes-unavailable fallback → Rule-vs-AI disagreement → offline/
// stale viewing → scheduler on/off toggle.
//
// Hermetic: the Yahoo fetcher is swapped for a deterministic stub via the
// fetchMarketDataFn seam, so no network is touched.

// stubPrices gives held tickers distinct prices so P&L math is assertable;
// every other ticker resolves to a default price so the whole universe refreshes.
var stubPrices = map[string]float64{
	"BBCA": 9750, "BBRI": 4600, "TLKM": 2900, "ASII": 5200,
}

// installStubFetcher replaces the live Yahoo fetcher with a deterministic one
// and registers cleanup to restore the real one. Returned data is a clean
// bullish snapshot (price > MA20 > MA50 > MA200, sound fundamentals) so every
// ticker gets scored and most land BUY/HOLD.
func installStubFetcher(t *testing.T) {
	t.Helper()
	orig := fetchMarketDataFn
	t.Cleanup(func() { fetchMarketDataFn = orig })
	fetchMarketDataFn = func(ticker string) (MarketData, error) {
		price, ok := stubPrices[ticker]
		if !ok {
			price = 5000
		}
		return MarketData{
			Ticker: ticker, CompanyName: "PT " + ticker + " Tbk",
			LastPrice: price, PrevClose: price * 0.99,
			PERatio: 15, PBVRatio: 2.0, ROE: 16, DER: 0.8, RevGrowth: 8, NetMargin: 12,
			MA20: price * 0.98, MA50: price * 0.97, MA200: price * 0.95,
			SourceURL: yahooQuoteURL(ticker),
		}, nil
	}
}

// --- in-process HTTP helpers ---

func doReq(t *testing.T, app *fiber.App, method, target string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, -1) // -1 = no timeout; in-process, fast
	if err != nil {
		t.Fatalf("%s %s: app.Test: %v", method, target, err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

func wantStatus(t *testing.T, resp *http.Response, want int, ctx string) {
	t.Helper()
	if resp.StatusCode != want {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s: status = %d, want %d (body=%s)", ctx, resp.StatusCode, want, b)
	}
}

// findPosition returns the portfolio row for ticker, failing the test if absent.
func findPosition(t *testing.T, pf portfolioResponse, ticker string) positionResponse {
	t.Helper()
	for _, p := range pf.Positions {
		if p.Ticker == ticker {
			return p
		}
	}
	t.Fatalf("ticker %s not in portfolio", ticker)
	return positionResponse{}
}

func TestEndToEnd(t *testing.T) {
	installStubFetcher(t)
	db := testDB(t)
	seedOpportunities(db)
	app := setupApp(db)
	held := map[string]bool{"BBCA": true, "BBRI": true, "TLKM": true, "ASII": true}

	// --- a. Seed sample portfolio (BBCA, BBRI, TLKM, ASII) ---
	t.Run("add positions", func(t *testing.T) {
		for _, p := range []createPositionRequest{
			{Ticker: "bbca", Shares: 1000, AvgBuyPrice: 9000, BuyDate: "2025-01-15"}, // lowercased → normalized to BBCA
			{Ticker: "BBRI", Shares: 500, AvgBuyPrice: 4400, BuyDate: "2025-03-02"},
			{Ticker: "TLKM", Shares: 2000, AvgBuyPrice: 3000, BuyDate: "2024-11-20"},
			{Ticker: "ASII", Shares: 300, AvgBuyPrice: 5500, BuyDate: "2025-06-10"},
		} {
			resp := doReq(t, app, "POST", "/api/v1/portfolio", p)
			wantStatus(t, resp, 201, "POST /portfolio "+p.Ticker)
			got := decode[Position](t, resp)
			if got.Ticker != strings.ToUpper(p.Ticker) {
				t.Fatalf("ticker not normalized: got %q", got.Ticker)
			}
		}
	})

	// --- b. Refresh → prices + scoring populated (no network) ---
	var refreshed refreshResponse
	t.Run("market-data refresh", func(t *testing.T) {
		resp := doReq(t, app, "POST", "/api/v1/market-data/refresh", nil)
		wantStatus(t, resp, 200, "POST /market-data/refresh")
		refreshed = decode[refreshResponse](t, resp)
		// 4 held + the rest of the universe all resolve via the stub → no failures.
		if refreshed.RefreshedCount < 4 {
			t.Fatalf("refreshed_count = %d, want ≥ 4", refreshed.RefreshedCount)
		}
		if refreshed.Failed != 0 {
			t.Fatalf("failed = %d, want 0 (stub fetcher never errors)", refreshed.Failed)
		}
		if refreshed.UpdatedAt == "" {
			t.Fatal("updated_at empty after refresh")
		}
	})

	// --- c. Portfolio: P&L + both-horizon rule verdicts ---
	t.Run("portfolio P&L and verdicts", func(t *testing.T) {
		resp := doReq(t, app, "GET", "/api/v1/portfolio", nil)
		wantStatus(t, resp, 200, "GET /portfolio")
		pf := decode[portfolioResponse](t, resp)

		if pf.Summary.CurrentValueIDR == nil {
			t.Fatal("summary.current_value_idr nil after refresh")
		}
		if pf.Summary.LastMarketUpdate == nil {
			t.Fatal("summary.last_market_update nil after refresh")
		}
		if len(pf.Positions) != 4 {
			t.Fatalf("positions = %d, want 4", len(pf.Positions))
		}

		// BBCA: 1000 × (9750 − 9000) = +750000.
		bbca := findPosition(t, pf, "BBCA")
		if bbca.CurrentPrice == nil || *bbca.CurrentPrice != 9750 {
			t.Fatalf("BBCA current_price = %v, want 9750", bbca.CurrentPrice)
		}
		if bbca.ProfitLossIDR == nil || *bbca.ProfitLossIDR != 750000 {
			t.Fatalf("BBCA profit_loss_idr = %v, want 750000", bbca.ProfitLossIDR)
		}
		if bbca.Verdicts == nil {
			t.Fatal("BBCA verdicts nil — scoring not populated")
		}
		for _, label := range []string{"short_term", "long_term"} {
			var vs verdictSet
			if label == "short_term" {
				vs = bbca.Verdicts.ShortTerm
			} else {
				vs = bbca.Verdicts.LongTerm
			}
			if vs.Rule == nil || *vs.Rule == "" {
				t.Fatalf("BBCA %s rule verdict empty", label)
			}
			if vs.AI != nil {
				t.Fatalf("BBCA %s AI should be nil before any Hermes run", label)
			}
		}
	})

	// --- d. Stock detail: score breakdown present + sums to score ---
	t.Run("stock detail breakdown", func(t *testing.T) {
		resp := doReq(t, app, "GET", "/api/v1/stocks/BBCA", nil)
		wantStatus(t, resp, 200, "GET /stocks/BBCA")
		d := decode[stockDetailResponse](t, resp)
		if d.MarketData.LastPrice != 9750 {
			t.Fatalf("detail last_price = %v, want 9750", d.MarketData.LastPrice)
		}
		if d.MarketData.UpdatedAt == "" {
			t.Fatal("detail market_data.updated_at empty (stale indicator needs it)")
		}
		for _, h := range []struct {
			name string
			lbl  string
			hz   horizonDTO
			want []string
		}{
			{"short_term", d.ShortTerm.HorizonLabel, d.ShortTerm, []string{"trend_teknis", "momentum", "volume", "valuasi", "earnings_momentum"}},
			{"long_term", d.LongTerm.HorizonLabel, d.LongTerm, []string{"profitabilitas", "solvabilitas", "valuasi", "pertumbuhan", "trend_teknis"}},
		} {
			if h.hz.Rule == nil {
				t.Fatalf("%s rule nil", h.name)
			}
			for _, k := range h.want {
				if _, ok := h.hz.Rule.Breakdown[k]; !ok {
					t.Fatalf("%s breakdown missing %q (%v)", h.name, k, h.hz.Rule.Breakdown)
				}
			}
			sum := 0
			for _, v := range h.hz.Rule.Breakdown {
				sum += v
			}
			if sum != h.hz.Rule.Score {
				t.Fatalf("%s breakdown sum = %d, want score %d", h.name, sum, h.hz.Rule.Score)
			}
		}
	})

	// --- e. Opportunities: LQ45/Kompas100 scored, ranked, held excluded ---
	t.Run("opportunities ranked universe", func(t *testing.T) {
		resp := doReq(t, app, "GET", "/api/v1/opportunities", nil)
		wantStatus(t, resp, 200, "GET /opportunities")
		ol := decode[opportunitiesResponse](t, resp)
		if len(ol.Opportunities) == 0 {
			t.Fatal("no scored opportunities — universe refresh not ranked")
		}
		for _, o := range ol.Opportunities {
			if held[o.Ticker] {
				t.Fatalf("held ticker %s appeared in opportunities", o.Ticker)
			}
			if o.ShortTermRule == "" || o.LongTermRule == "" {
				t.Fatalf("opportunity %s missing verdicts", o.Ticker)
			}
		}
		// Buy-first ranking: verdict rank desc, then short score desc.
		for i := 1; i < len(ol.Opportunities); i++ {
			a, b := ol.Opportunities[i-1], ol.Opportunities[i]
			ra, rb := verdictRank(a.ShortTermRule), verdictRank(b.ShortTermRule)
			if ra < rb || (ra == rb && a.ShortTermScore < b.ShortTermScore) {
				t.Fatalf("opportunities not ranked at %d: %v before %v", i, a.Ticker, b.Ticker)
			}
		}

		// filter=lq45 keeps only LQ45/BOTH members.
		resp = doReq(t, app, "GET", "/api/v1/opportunities?filter=lq45", nil)
		wantStatus(t, resp, 200, "GET /opportunities?filter=lq45")
		ol = decode[opportunitiesResponse](t, resp)
		for _, o := range ol.Opportunities {
			if o.IndexMembership != idxLQ45 && o.IndexMembership != idxBoth {
				t.Fatalf("lq45 filter leaked %s (%s)", o.Ticker, o.IndexMembership)
			}
		}

		// Custom-ticker lookup: out-of-universe → illiquid.
		resp = doReq(t, app, "GET", "/api/v1/opportunities/lookup?q=ZZZZ", nil)
		wantStatus(t, resp, 200, "GET /opportunities/lookup?q=ZZZZ")
		lr := decode[lookupResponse](t, resp)
		if lr.InUniverse || !lr.Illiquid {
			t.Fatalf("unknown ticker lookup = %+v, want illiquid", lr)
		}
	})

	// --- f. Settings GET/PUT round-trip + scheduler config persists ---
	t.Run("settings round-trip", func(t *testing.T) {
		resp := doReq(t, app, "GET", "/api/v1/settings", nil)
		wantStatus(t, resp, 200, "GET /settings")
		got := decode[settingsResponse](t, resp)
		if got.BackgroundRefreshEnabled {
			t.Fatal("default background_refresh_enabled should be false")
		}
		if got.RefreshIntervalHours != 24 {
			t.Fatalf("default interval = %d, want 24", got.RefreshIntervalHours)
		}

		resp = doReq(t, app, "PUT", "/api/v1/settings", updateSettingsRequest{
			BackgroundRefreshEnabled: ptr(true),
			RefreshIntervalHours:     ptr(12),
			HermesExecutable:         ptr("hermes"),
		})
		wantStatus(t, resp, 200, "PUT /settings")
		upd := decode[settingsResponse](t, resp)
		if !upd.BackgroundRefreshEnabled || upd.RefreshIntervalHours != 12 {
			t.Fatalf("settings not updated: %+v", upd)
		}

		// Re-read → persisted to SQLite.
		resp = doReq(t, app, "GET", "/api/v1/settings", nil)
		got = decode[settingsResponse](t, resp)
		if !got.BackgroundRefreshEnabled || got.RefreshIntervalHours != 12 {
			t.Fatalf("settings not persisted: %+v", got)
		}
	})

	// --- g. Hermes-unavailable fallback: no crash, clear state ---
	t.Run("hermes unavailable fallback", func(t *testing.T) {
		doReq(t, app, "PUT", "/api/v1/settings", updateSettingsRequest{
			HermesExecutable: ptr("/no/such/hermes-binary"),
		})
		resp := doReq(t, app, "POST", "/api/v1/stocks/BBCA/ai-analyze", nil)
		wantStatus(t, resp, 200, "POST /stocks/BBCA/ai-analyze (unavailable)") // 200, not 500
		ar := decode[aiAnalyzeResponse](t, resp)
		if ar.Status != "unavailable" {
			t.Fatalf("status = %q, want unavailable (no crash)", ar.Status)
		}
		if ar.Detail != nil {
			t.Fatalf("expected no detail on unavailable, got %+v", ar.Detail)
		}

		// Fallback stored nothing: portfolio still has no AI verdict.
		pf := decode[portfolioResponse](t, doReq(t, app, "GET", "/api/v1/portfolio", nil))
		bbca := findPosition(t, pf, "BBCA")
		if bbca.Verdicts != nil && bbca.Verdicts.ShortTerm.AI != nil {
			t.Fatal("BBCA short AI populated despite unavailable Hermes")
		}
	})

	// --- h. Rule-vs-AI via stub binary → disagreement rendered ---
	t.Run("rule-vs-AI disagreement", func(t *testing.T) {
		// BBRI short rule scores BUY (bullish stub data); stub AI says HOLD → disagreement.
		stub := writeHermesStub(t, `{"short_term":{"verdict":"HOLD","confidence":0.6,"reasoning":"valuasi cuff"},"long_term":{"verdict":"HOLD","confidence":0.55,"reasoning":"cukup"},"risk_factors":["koreksi"],"data_limitations":[]}`)
		doReq(t, app, "PUT", "/api/v1/settings", updateSettingsRequest{HermesExecutable: ptr(stub)})

		resp := doReq(t, app, "POST", "/api/v1/stocks/BBRI/ai-analyze", nil)
		wantStatus(t, resp, 200, "POST /stocks/BBRI/ai-analyze (stub)")
		ar := decode[aiAnalyzeResponse](t, resp)
		if ar.Status != "done" {
			t.Fatalf("status = %q, want done", ar.Status)
		}
		if ar.Detail == nil || ar.Detail.ShortTerm.AI == nil {
			t.Fatal("AI side missing from returned detail")
		}

		// Portfolio now surfaces AI verdict + disagreement flag.
		pf := decode[portfolioResponse](t, doReq(t, app, "GET", "/api/v1/portfolio", nil))
		bbri := findPosition(t, pf, "BBRI")
		if bbri.Verdicts == nil {
			t.Fatal("BBRI verdicts nil")
		}
		if bbri.Verdicts.ShortTerm.AI == nil || *bbri.Verdicts.ShortTerm.AI != verdictHold {
			t.Fatalf("BBRI short AI = %v, want HOLD", bbri.Verdicts.ShortTerm.AI)
		}
		if bbri.Verdicts.ShortTerm.Rule == nil || *bbri.Verdicts.ShortTerm.Rule != verdictBuy {
			t.Fatalf("BBRI short rule = %v, want BUY (precondition for disagreement)", bbri.Verdicts.ShortTerm.Rule)
		}
		if !bbri.Verdicts.ShortTerm.Disagreement {
			t.Fatal("BBRI short disagreement flag false — Rule BUY vs AI HOLD must disagree")
		}

		// 24h cache: a second call is served from the store, not the stub.
		resp = doReq(t, app, "POST", "/api/v1/stocks/BBRI/ai-analyze", nil)
		ar = decode[aiAnalyzeResponse](t, resp)
		if ar.Status != "cached" {
			t.Fatalf("second ai-analyze status = %q, want cached", ar.Status)
		}
	})

	// --- i. Offline viewing + stale-data indicator ---
	t.Run("offline stale viewing", func(t *testing.T) {
		// Push TLKM's market data 48h into the past to simulate stale/offline state.
		stale := time.Now().UTC().Add(-48 * time.Hour)
		if err := db.Model(&MarketData{}).Where("ticker = ?", "TLKM").
			Update("updated_at", stale).Error; err != nil {
			t.Fatalf("age TLKM data: %v", err)
		}

		// GET still serves full data from the store (offline-readable)…
		resp := doReq(t, app, "GET", "/api/v1/stocks/TLKM", nil)
		wantStatus(t, resp, 200, "GET /stocks/TLKM (stale)")
		d := decode[stockDetailResponse](t, resp)
		if d.MarketData.LastPrice <= 0 {
			t.Fatal("stale TLKM returned no price — offline viewing broken")
		}
		// …and surfaces the old timestamp so the UI can render a stale indicator.
		got, err := time.Parse(time.RFC3339, d.MarketData.UpdatedAt)
		if err != nil {
			t.Fatalf("parse TLKM updated_at %q: %v", d.MarketData.UpdatedAt, err)
		}
		if time.Since(got) < 24*time.Hour {
			t.Fatalf("TLKM updated_at = %v, want >24h old (stale)", got)
		}
	})

	// --- j. Scheduler on/off toggle verified against runScheduledRefresh ---
	t.Run("scheduler toggle", func(t *testing.T) {
		// Enabled + overdue → runScheduledRefresh refreshes and stamps last run.
		doReq(t, app, "PUT", "/api/v1/settings", updateSettingsRequest{
			BackgroundRefreshEnabled: ptr(true),
			RefreshIntervalHours:     ptr(1),
		})
		var tlkm MarketData
		if err := db.First(&tlkm, "ticker = ?", "TLKM").Error; err != nil {
			t.Fatal(err)
		}
		beforeUpdate := tlkm.UpdatedAt

		runScheduledRefresh(db) // enabled + (no last run yet) → due → refreshes
		s, _ := loadSettings(db)
		if s.LastBackgroundRefresh == nil {
			t.Fatal("last_background_refresh nil after enabled scheduled refresh")
		}
		if err := db.First(&tlkm, "ticker = ?", "TLKM").Error; err != nil {
			t.Fatal(err)
		}
		if !tlkm.UpdatedAt.After(beforeUpdate) {
			t.Fatal("TLKM not re-refreshed by enabled scheduler")
		}

		// Disabled → runScheduledRefresh returns early; data untouched.
		stable := tlkm.UpdatedAt
		doReq(t, app, "PUT", "/api/v1/settings", updateSettingsRequest{
			BackgroundRefreshEnabled: ptr(false),
		})
		runScheduledRefresh(db)
		if err := db.First(&tlkm, "ticker = ?", "TLKM").Error; err != nil {
			t.Fatal(err)
		}
		if !tlkm.UpdatedAt.Equal(stable) {
			t.Fatal("scheduler refreshed despite background refresh disabled")
		}
	})
}

// writeHermesStub, ptr[T], testDB, seedOpportunities, verdictRank/verdictBuy/
// verdictHold and the DTO types are defined in the other _test.go / source
// files of this package — reused here without redeclaration.

