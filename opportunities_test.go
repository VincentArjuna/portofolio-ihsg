package main

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testDB opens a fresh in-memory SQLite store with all models migrated.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("buka db: %v", err)
	}
	if err := db.AutoMigrate(&Position{}, &MarketData{}, &AnalysisResult{}, &AppSettings{}, &Opportunity{}); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	return db
}

func TestIndexMembership(t *testing.T) {
	cases := []struct{ lq45, kompas bool; want string }{
		{true, true, idxBoth},
		{true, false, idxLQ45},
		{false, true, idxKompas},
		{false, false, ""},
	}
	for _, c := range cases {
		if got := indexMembership(c.lq45, c.kompas); got != c.want {
			t.Fatalf("indexMembership(%v,%v) = %q, want %q", c.lq45, c.kompas, got, c.want)
		}
	}
}

func TestInIndex(t *testing.T) {
	cases := []struct {
		membership, filter string
		want               bool
	}{
		{idxBoth, "lq45", true},
		{idxBoth, "kompas100", true},
		{idxBoth, "both", true},
		{idxLQ45, "lq45", true},
		{idxLQ45, "kompas100", false},
		{idxLQ45, "both", false},
		{idxKompas, "kompas100", true},
		{idxKompas, "lq45", false},
		{idxKompas, "", true}, // empty filter = all
		{idxKompas, "all", true},
		{idxKompas, "KOMPAS100", true}, // uppercase filter is case-insensitive
	}
	for _, c := range cases {
		if got := inIndex(c.membership, c.filter); got != c.want {
			t.Fatalf("inIndex(%q,%q) = %v, want %v", c.membership, c.filter, got, c.want)
		}
	}
}

func TestVerdictRank(t *testing.T) {
	cases := map[string]int{
		verdictBuy: 2, verdictHold: 1, verdictSell: 0, "": -1, "WEIRD": -1,
	}
	for v, want := range cases {
		if got := verdictRank(v); got != want {
			t.Fatalf("verdictRank(%q) = %d, want %d", v, got, want)
		}
	}
}

func TestMeetsMinVerdict(t *testing.T) {
	cases := []struct {
		verdict, min string
		want         bool
	}{
		{verdictBuy, "BUY", true},
		{verdictHold, "BUY", false}, // HOLD < BUY
		{verdictSell, "HOLD", false},
		{verdictSell, "", true},     // no filter
		{verdictBuy, "garbage", true}, // unknown filter → no-op
	}
	for _, c := range cases {
		if got := meetsMinVerdict(c.verdict, c.min); got != c.want {
			t.Fatalf("meetsMinVerdict(%q,%q) = %v, want %v", c.verdict, c.min, got, c.want)
		}
	}
}

// TestSortOpportunities_BuyFirst: BUY groups before HOLD before SELL, scores
// descend within a group, ticker breaks ties.
func TestSortOpportunities_BuyFirst(t *testing.T) {
	opps := []opportunityResponse{
		{Ticker: "CCCC", ShortTermRule: verdictSell, ShortTermScore: 20},
		{Ticker: "BBBB", ShortTermRule: verdictHold, ShortTermScore: 50},
		{Ticker: "AAAA", ShortTermRule: verdictBuy, ShortTermScore: 80},
		{Ticker: "DDDD", ShortTermRule: verdictBuy, ShortTermScore: 90},
		{Ticker: "ZZZZ", ShortTermRule: verdictBuy, ShortTermScore: 90}, // tie → ticker asc (DDDD before ZZZZ)
	}
	sortOpportunities(opps)
	want := []string{"DDDD", "ZZZZ", "AAAA", "BBBB", "CCCC"}
	for i, o := range opps {
		if o.Ticker != want[i] {
			t.Fatalf("pos %d = %q, want %q (full order: %v)", i, o.Ticker, want[i], tickerOrder(opps))
		}
	}
}

func tickerOrder(o []opportunityResponse) []string {
	out := make([]string, len(o))
	for i, v := range o {
		out[i] = v.Ticker
	}
	return out
}

// ar builds a both-horizon verdict pair for one ticker.
func ar(ticker, verdict string, score int) (short, long AnalysisResult) {
	short = AnalysisResult{ID: ticker + "-s", Ticker: ticker, Horizon: horizonShort, RuleVerdict: verdict, RuleScore: score}
	long = AnalysisResult{ID: ticker + "-l", Ticker: ticker, Horizon: horizonLong, RuleVerdict: verdict, RuleScore: score + 5}
	return short, long
}

// scoredSeed inserts an opportunity + market data + both-horizon verdicts.
func scoredSeed(t *testing.T, db *gorm.DB, ticker, membership string, verdict string, score int) {
	t.Helper()
	if err := db.Create(&Opportunity{Ticker: ticker, IndexMembership: membership, Sector: "Sektor", Name: ticker}).Error; err != nil {
		t.Fatalf("create opp %s: %v", ticker, err)
	}
	if err := db.Create(&MarketData{Ticker: ticker, LastPrice: 100, CompanyName: ticker, PERatio: 12, ROE: 15}).Error; err != nil {
		t.Fatalf("create md %s: %v", ticker, err)
	}
	short, long := ar(ticker, verdict, score)
	if err := db.Create(&short).Error; err != nil {
		t.Fatalf("create short %s: %v", ticker, err)
	}
	if err := db.Create(&long).Error; err != nil {
		t.Fatalf("create long %s: %v", ticker, err)
	}
}

func TestBuildOpportunities(t *testing.T) {
	db := testDB(t)

	scoredSeed(t, db, "AAAA", idxBoth, verdictBuy, 80)   // held → excluded
	scoredSeed(t, db, "BBBB", idxLQ45, verdictHold, 50)
	scoredSeed(t, db, "CCCC", idxKompas, verdictSell, 20)
	scoredSeed(t, db, "DDDD", idxBoth, verdictBuy, 90)    // top Buy
	// EEEE is in the universe but unscored (no market data) → omitted.
	if err := db.Create(&Opportunity{Ticker: "EEEE", IndexMembership: idxBoth, Sector: "S", Name: "EEEE"}).Error; err != nil {
		t.Fatalf("create EEEE: %v", err)
	}
	// Mark AAAA held.
	if err := db.Create(&Position{ID: "p1", Ticker: "AAAA", Shares: 10, AvgBuyPrice: 100, BuyDate: "2026-01-01"}).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}

	// No filter: Buy-first, AAAA excluded, EEEE omitted.
	opps, err := buildOpportunities(db, "", "", "")
	if err != nil {
		t.Fatalf("buildOpportunities: %v", err)
	}
	got := tickerOrder(opps)
	want := []string{"DDDD", "BBBB", "CCCC"}
	if len(got) != len(want) {
		t.Fatalf("order len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i, tk := range got {
		if tk != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, tk, want[i], got)
		}
	}

	// filter=lq45 → only LQ45+BOTH (DDDD, BBBB); CCCC dropped, AAAA held-excluded.
	opps, _ = buildOpportunities(db, "lq45", "", "")
	if got := tickerOrder(opps); len(got) != 2 || got[0] != "DDDD" || got[1] != "BBBB" {
		t.Fatalf("lq45 filter = %v, want [DDDD BBBB]", got)
	}

	// filter=kompas100 → DDDD + CCCC.
	opps, _ = buildOpportunities(db, "kompas100", "", "")
	if got := tickerOrder(opps); len(got) != 2 || got[0] != "DDDD" || got[1] != "CCCC" {
		t.Fatalf("kompas100 filter = %v, want [DDDD CCCC]", got)
	}

	// min_verdict=BUY → only DDDD.
	opps, _ = buildOpportunities(db, "", "BUY", "")
	if got := tickerOrder(opps); len(got) != 1 || got[0] != "DDDD" {
		t.Fatalf("min_verdict=BUY = %v, want [DDDD]", got)
	}

	// q=dd → substring match on ticker/name.
	opps, _ = buildOpportunities(db, "", "", "dd")
	if got := tickerOrder(opps); len(got) != 1 || got[0] != "DDDD" {
		t.Fatalf("q=dd = %v, want [DDDD]", got)
	}
}

func TestSeedOpportunities_Idempotent(t *testing.T) {
	db := testDB(t)
	seedOpportunities(db)
	seedOpportunities(db) // re-seed must not duplicate

	var n int64
	db.Model(&Opportunity{}).Count(&n)
	if n != int64(len(seedUniverse)) {
		t.Fatalf("opportunity count = %d, want %d", n, len(seedUniverse))
	}

	// Spot-check membership tokens for known tickers.
	sample := map[string]string{"BBCA": idxBoth, "ACES": idxKompas, "ESSA": idxLQ45}
	for ticker, want := range sample {
		var opp Opportunity
		if err := db.First(&opp, "ticker = ?", ticker).Error; err != nil {
			t.Fatalf("seed missing %s: %v", ticker, err)
		}
		if opp.IndexMembership != want {
			t.Fatalf("%s membership = %q, want %q", ticker, opp.IndexMembership, want)
		}
	}
}
