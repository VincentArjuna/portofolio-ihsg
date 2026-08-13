package main

import (
	"sort"
	"testing"
)

// mdBuy/mHold/mSell are fixed snapshots whose scores were derived by hand so
// the verdict bands and integer breakdowns are pinned. Adjust scoring math →
// re-derive the expected ints here.

func mdBuy() MarketData {
	return MarketData{
		Ticker: "BBCA", LastPrice: 9750, PrevClose: 9500,
		PERatio: 18, PBVRatio: 3.0, ROE: 18, DER: 0.3, RevGrowth: 12, NetMargin: 24,
		MA20: 9600, MA50: 9400, MA200: 9000,
	}
}

func mdHold() MarketData {
	return MarketData{
		Ticker: "HOLD", LastPrice: 5000, PrevClose: 4950,
		PERatio: 22, PBVRatio: 2.5, ROE: 10, DER: 1.2, RevGrowth: 4, NetMargin: 8,
		MA20: 4950, MA50: 5000, MA200: 4800,
	}
}

func mdSell() MarketData {
	return MarketData{
		Ticker: "LOSE", LastPrice: 1000, PrevClose: 1100,
		PERatio: 40, PBVRatio: 5.0, ROE: 2, DER: 3.0, RevGrowth: -15, NetMargin: -8,
		MA20: 1100, MA50: 1200, MA200: 1400,
	}
}

// assertBreakdownSums verifies the invariant: each breakdown contribution
// rounds to an int and they sum exactly to score (docs/api-contract.md shape).
func assertBreakdownSums(t *testing.T, score int, bd map[string]int) {
	t.Helper()
	sum := 0
	for _, v := range bd {
		sum += v
	}
	if sum != score {
		t.Fatalf("breakdown sum = %d, want score %d", sum, score)
	}
}

func TestVerdictFromScore(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, verdictBuy}, {65, verdictBuy},
		{64, verdictHold}, {40, verdictHold},
		{39, verdictSell}, {0, verdictSell},
	}
	for _, c := range cases {
		if got := verdictFromScore(c.score); got != c.want {
			t.Fatalf("verdictFromScore(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestScoreShort_Bands(t *testing.T) {
	cases := []struct {
		name   string
		md     MarketData
		score  int
		verdict string
	}{
		{"buy", mdBuy(), 78, verdictBuy},
		{"hold", mdHold(), 49, verdictHold},
		{"sell", mdSell(), 16, verdictSell},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			score, bd := scoreShort(c.md)
			if score != c.score {
				t.Fatalf("short score = %d, want %d (bd=%v)", score, c.score, bd)
			}
			if got := verdictFromScore(score); got != c.verdict {
				t.Fatalf("short verdict = %q, want %q", got, c.verdict)
			}
			// All five factors present.
			for _, k := range []string{"trend_teknis", "momentum", "volume", "valuasi", "earnings_momentum"} {
				if _, ok := bd[k]; !ok {
					t.Fatalf("short breakdown missing %q (bd=%v)", k, bd)
				}
			}
			assertBreakdownSums(t, score, bd)
		})
	}
}

func TestScoreLong_Bands(t *testing.T) {
	cases := []struct {
		name   string
		md     MarketData
		score  int
		verdict string
	}{
		{"buy", mdBuy(), 82, verdictBuy},
		{"hold", mdHold(), 54, verdictHold},
		{"sell", mdSell(), 19, verdictSell},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			score, bd := scoreLong(c.md)
			if score != c.score {
				t.Fatalf("long score = %d, want %d (bd=%v)", score, c.score, bd)
			}
			if got := verdictFromScore(score); got != c.verdict {
				t.Fatalf("long verdict = %q, want %q", got, c.verdict)
			}
			for _, k := range []string{"profitabilitas", "solvabilitas", "valuasi", "pertumbuhan", "trend_teknis"} {
				if _, ok := bd[k]; !ok {
					t.Fatalf("long breakdown missing %q (bd=%v)", k, bd)
				}
			}
			assertBreakdownSums(t, score, bd)
		})
	}
}

func TestComputeRiskFlags(t *testing.T) {
	got := computeRiskFlags(mdSell())
	want := []string{"high_debt", "low_profitability", "overvalued", "downtrend"}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("risk flags = %v, want all four %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("risk flags = %v, want %v", got, want)
		}
	}

	// Healthy snapshot → no flags.
	if f := computeRiskFlags(mdBuy()); len(f) != 0 {
		t.Fatalf("buy snapshot risk flags = %v, want none", f)
	}
}

// ROE==0 means "missing", not "low": must not raise low_profitability.
func TestComputeRiskFlags_MissingROENotLow(t *testing.T) {
	md := MarketData{ROE: 0, DER: 0.5, PERatio: 10, PBVRatio: 1, LastPrice: 100, MA50: 90, MA200: 80}
	if f := computeRiskFlags(md); len(f) != 0 {
		t.Fatalf("missing-ROE snapshot flags = %v, want none", f)
	}
}

func TestAnalyzeTicker_BothHorizons(t *testing.T) {
	short, long := analyzeTicker(mdBuy())
	if short.Horizon != horizonShort || long.Horizon != horizonLong {
		t.Fatalf("horizons = %q/%q", short.Horizon, long.Horizon)
	}
	if short.RuleScore != 78 || long.RuleScore != 82 {
		t.Fatalf("scores = %d/%d, want 78/82", short.RuleScore, long.RuleScore)
	}
	if short.RuleVerdict != verdictBuy || long.RuleVerdict != verdictBuy {
		t.Fatalf("verdicts = %q/%q, want BUY/BUY", short.RuleVerdict, long.RuleVerdict)
	}
	// Risk flags are ticker-level → identical across horizons.
	if short.RiskFlags != long.RiskFlags {
		t.Fatalf("risk flags differ across horizons: %q vs %q", short.RiskFlags, long.RiskFlags)
	}
}
