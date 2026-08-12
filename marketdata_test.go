package main

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// buildChartJSON assembles a Yahoo chart payload with the given meta price and
// daily closes, so parseChartBody can be exercised without network access.
func buildChartJSON(t *testing.T, lastPrice float64, closes []float64) []byte {
	t.Helper()
	closesJSON, err := json.Marshal(closes)
	if err != nil {
		t.Fatalf("marshal closes: %v", err)
	}
	tpl := `{"chart":{"result":[{"meta":{"regularMarketPrice":%g,"chartPreviousClose":6000,` +
		`"longName":"PT Bank Central Asia Tbk","shortName":"BCA"},"timestamp":[],` +
		`"indicators":{"quote":[{"close":%s}]}}]}}`
	return []byte(fmt.Sprintf(tpl, lastPrice, closesJSON))
}

func TestMA(t *testing.T) {
	// closes = [1..200]
	c := make([]float64, 200)
	for i := range c {
		c[i] = float64(i + 1)
	}
	if got := ma(c, 200); got != 100.5 {
		t.Fatalf("ma200 = %v, want 100.5", got)
	}
	if got := ma(c, 20); got != 190.5 { // mean(181..200)
		t.Fatalf("ma20 = %v, want 190.5", got)
	}
	if got := ma(c, 250); got != 0 { // insufficient history
		t.Fatalf("ma250 = %v, want 0", got)
	}
	if got := ma(nil, 20); got != 0 {
		t.Fatalf("ma(nil) = %v, want 0", got)
	}
}

func TestParseChartBody_ClosedMarket(t *testing.T) {
	// Last close == live price → market closed; prev_close must be prior bar.
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 100
	}
	closes[29] = 6300 // today's close == live price
	closes[28] = 6175 // yesterday
	body := buildChartJSON(t, 6300, closes)

	md, err := parseChartBody(body, "BBCA")
	if err != nil {
		t.Fatalf("parseChartBody: %v", err)
	}
	if md.LastPrice != 6300 {
		t.Fatalf("last_price = %v, want 6300", md.LastPrice)
	}
	if md.PrevClose != 6175 {
		t.Fatalf("prev_close = %v, want 6175 (prior bar)", md.PrevClose)
	}
	if md.CompanyName != "PT Bank Central Asia Tbk" {
		t.Fatalf("company_name = %q", md.CompanyName)
	}
	// MA20 = mean of last 20 closes (indices 10..29): eighteen 100s + 6175 + 6300.
	wantMA20 := (18*100 + 6175 + 6300) / 20.0
	if md.MA20 == 0 || math.Abs(md.MA20-wantMA20) > 1e-6 {
		t.Fatalf("ma20 = %v, want %v", md.MA20, wantMA20)
	}
	if md.MA200 != 0 {
		t.Fatalf("ma200 = %v, want 0 (insufficient history)", md.MA200)
	}
	if md.SourceURL != "https://finance.yahoo.com/quote/BBCA.JK" {
		t.Fatalf("source_url = %q", md.SourceURL)
	}
}

func TestParseChartBody_Intraday(t *testing.T) {
	// Live price differs from last close → intraday; prev_close = last close.
	closes := []float64{100, 110, 6175}
	body := buildChartJSON(t, 6300, closes) // 6300 != 6175

	md, err := parseChartBody(body, "BBRI")
	if err != nil {
		t.Fatalf("parseChartBody: %v", err)
	}
	if md.PrevClose != 6175 {
		t.Fatalf("prev_close = %v, want 6175 (last completed close)", md.PrevClose)
	}
}

func TestParseQuoteSummaryBody(t *testing.T) {
	body := []byte(`{"quoteSummary":{"result":[{
		"summaryDetail":{"trailingPE":{"raw":18.5}},
		"defaultKeyStatistics":{"priceToBook":{"raw":3.2}},
		"financialData":{
			"returnOnEquity":{"raw":0.182},
			"debtToEquity":{"raw":23.6},
			"revenueGrowth":{"raw":0.124},
			"profitMargins":{"raw":0.241}
		}
	}]}}`)

	md := MarketData{}
	if err := parseQuoteSummaryBody(body, &md); err != nil {
		t.Fatalf("parseQuoteSummaryBody: %v", err)
	}
	cases := []struct {
		name         string
		got, want    float64
	}{
		{"PER", md.PERatio, 18.5},
		{"PBV", md.PBVRatio, 3.2},
		{"ROE", md.ROE, 18.2}, // 0.182 → 18.2%
		{"DER", md.DER, 0.236}, // 23.6 → 0.236
		{"RevGrowth", md.RevGrowth, 12.4},
		{"NetMargin", md.NetMargin, 24.1},
	}
	for _, c := range cases {
		if math.Abs(c.got-c.want) > 1e-6 {
			t.Fatalf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestParseQuoteSummaryBody_ForwardPEFallback(t *testing.T) {
	body := []byte(`{"quoteSummary":{"result":[{
		"summaryDetail":{},
		"defaultKeyStatistics":{"forwardPE":{"raw":16.0}},
		"financialData":{}
	}]}}`)
	md := MarketData{}
	if err := parseQuoteSummaryBody(body, &md); err != nil {
		t.Fatalf("parseQuoteSummaryBody: %v", err)
	}
	if md.PERatio != 16.0 {
		t.Fatalf("PER = %v, want 16.0 (forwardPE fallback)", md.PERatio)
	}
}

func TestParseChartBody_Error(t *testing.T) {
	body := []byte(`{"chart":{"error":{"description":"Invalid Symbol"}}}`)
	if _, err := parseChartBody(body, "NOPE"); err == nil {
		t.Fatal("expected error for bad symbol")
	}
}
