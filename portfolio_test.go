package main

import "testing"

// portfolio_test.go — issue #16: fractional shares + decimal avg_buy_price.
//
// Verifies a decimal position round-trips through the API and that cost basis
// computes exactly with float64 shares. Helpers (testDB, setupApp, doReq,
// decode, wantStatus, findPosition) live in the sibling _test.go files.

func TestDecimalShares(t *testing.T) {
	db := testDB(t)
	app := setupApp(db)

	// shares=500.5 @ avg 3147.75 → cost basis 1,574,823.75.
	resp := doReq(t, app, "POST", "/api/v1/portfolio", createPositionRequest{
		Ticker: "BBCA", Shares: 500.5, AvgBuyPrice: 3147.75, BuyDate: "2025-01-15",
	})
	wantStatus(t, resp, 201, "POST decimal shares")
	got := decode[Position](t, resp)
	if got.Shares != 500.5 {
		t.Fatalf("shares round-trip = %v, want 500.5", got.Shares)
	}
	if got.AvgBuyPrice != 3147.75 {
		t.Fatalf("avg_buy_price round-trip = %v, want 3147.75", got.AvgBuyPrice)
	}

	// Cost basis = shares × avg, summed across positions.
	pf := decode[portfolioResponse](t, doReq(t, app, "GET", "/api/v1/portfolio", nil))
	wantCost := 500.5 * 3147.75
	if pf.Summary.TotalInvestmentIDR != wantCost {
		t.Fatalf("cost basis = %v, want %v", pf.Summary.TotalInvestmentIDR, wantCost)
	}
	if bbca := findPosition(t, pf, "BBCA"); bbca.Shares != 500.5 {
		t.Fatalf("portfolio shares = %v, want 500.5", bbca.Shares)
	}

	// Positive decimals accepted; non-positive still rejected.
	wantStatus(t, doReq(t, app, "POST", "/api/v1/portfolio", createPositionRequest{
		Ticker: "BBRI", Shares: 0.5, AvgBuyPrice: 4400, BuyDate: "2025-01-15",
	}), 201, "POST 0.5 shares valid")
	wantStatus(t, doReq(t, app, "POST", "/api/v1/portfolio", createPositionRequest{
		Ticker: "TLKM", Shares: 0, AvgBuyPrice: 3000, BuyDate: "2025-01-15",
	}), 400, "POST 0 shares rejected")
}
