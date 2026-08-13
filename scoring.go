package main

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnalysisResult is the persisted rule-based verdict for one ticker + horizon.
// One row per (ticker, horizon) with horizon ∈ {"short","long"}. rule_breakdown
// and risk_flags are stored as JSON strings (SQLite TEXT) so no GORM serializer
// dependency is needed. AI* fields are populated by the T4 Hermes bridge (nil/
// zero until "Analisis AI" runs); GORM AutoMigrate extends this table then
// without data loss.
type AnalysisResult struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	Ticker        string     `gorm:"index;not null" json:"ticker"`
	Horizon       string     `gorm:"index;not null" json:"horizon"` // "short" | "long"
	RuleVerdict   string     `json:"rule_verdict"`                  // "BUY" | "HOLD" | "SELL"
	RuleScore     int        `json:"rule_score"`                    // 0-100
	RuleBreakdown string     `json:"rule_breakdown"`                // JSON: {"trend_teknis":25,...}
	RiskFlags     string     `json:"risk_flags"`                    // JSON: ["high_debt",...]
	AIVerdict     string     `json:"ai_verdict"`                    // "" until T4 run; BUY|HOLD|SELL
	AIExplanation string     `json:"ai_explanation"`                // Indonesian reasoning + limitations
	AIConfidence  float64    `json:"ai_confidence"`                 // 0.0-1.0
	AIRiskFactors string     `json:"ai_risk_factors"`               // JSON: ["...","..."]
	AIUpdatedAt   *time.Time `json:"ai_updated_at"`                 // nil until T4 run; drives 24h cache
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

const (
	horizonShort = "short"
	horizonLong  = "long"

	verdictBuy  = "BUY"
	verdictHold = "HOLD"
	verdictSell = "SELL"

	horizonShortLabel = "6-12 Bulan"
	horizonLongLabel  = "3-5 Tahun"
)

// verdictFromScore maps a 0-100 score to a verdict per docs/domain.md:
// ≥65 → Buy, 40-64 → Hold, <40 → Sell.
func verdictFromScore(score int) string {
	switch {
	case score >= 65:
		return verdictBuy
	case score >= 40:
		return verdictHold
	default:
		return verdictSell
	}
}

// scoreComponent is one weighted factor: frac ∈ [0,1] is the fraction of
// `weight` the ticker earns. The integer contribution is round(frac*weight).
type scoreComponent struct {
	key    string
	weight int
	frac   float64
}

// assemble rounds each component's contribution and sums them. Because score
// is the sum of the rounded breakdown values, breakdown always sums exactly to
// score (matches docs/api-contract.md examples). Total is capped at 100.
func assemble(cs []scoreComponent) (int, map[string]int) {
	breakdown := make(map[string]int, len(cs))
	score := 0
	for _, c := range cs {
		v := int(math.Round(c.frac * float64(c.weight)))
		breakdown[c.key] = v
		score += v
	}
	if score > 100 {
		score = 100
	}
	return score, breakdown
}

// ---- Shared sub-scorers (0.0-1.0 fractions) ----

func perFraction(md MarketData) float64 {
	if md.PERatio <= 0 {
		return 0.5 // no data → neutral
	}
	switch {
	case md.PERatio <= 10:
		return 1.0
	case md.PERatio <= 15:
		return 0.8
	case md.PERatio <= 20:
		return 0.6
	case md.PERatio <= 25:
		return 0.4
	default:
		return 0.2
	}
}

func pbvFraction(md MarketData) float64 {
	if md.PBVRatio <= 0 {
		return 0.5
	}
	switch {
	case md.PBVRatio <= 1:
		return 1.0
	case md.PBVRatio <= 2:
		return 0.75
	case md.PBVRatio <= 3:
		return 0.5
	default:
		return 0.25
	}
}

func valuasiFraction(md MarketData) float64 {
	return (perFraction(md) + pbvFraction(md)) / 2
}

func netMarginFraction(md MarketData) float64 {
	switch {
	case md.NetMargin >= 20:
		return 1.0
	case md.NetMargin >= 10:
		return 0.7
	case md.NetMargin >= 0:
		return 0.4
	default:
		return 0.1 // negative margin penalized (Balanced-Growth)
	}
}

// ---- Short-term (6-12 bulan): technical-led ----

// trendTeknisFraction scores MA-stack alignment (price vs MA20/50/200).
func trendTeknisFraction(md MarketData) float64 {
	if md.MA20 == 0 || md.MA50 == 0 || md.MA200 == 0 {
		return 0.5 // insufficient history → neutral (data limitation)
	}
	switch {
	case md.LastPrice > md.MA20 && md.MA20 > md.MA50 && md.MA50 > md.MA200:
		return 1.0 // full bullish alignment
	case md.LastPrice > md.MA20 && md.MA20 > md.MA50:
		return 0.75
	case md.LastPrice > md.MA20:
		return 0.5
	case md.LastPrice > md.MA50:
		return 0.3
	default:
		return 0.1
	}
}

// momentumFraction is a single-bar rate-of-change proxy from prev_close. The
// MarketData model only carries last_price + prev_close (T2), so this is the
// momentum signal the available data supports.
func momentumFraction(md MarketData) float64 {
	if md.PrevClose <= 0 {
		return 0.5
	}
	roc := (md.LastPrice - md.PrevClose) / md.PrevClose
	switch {
	case roc >= 0.05:
		return 1.0
	case roc >= 0.02:
		return 0.75
	case roc >= 0:
		return 0.5
	case roc >= -0.02:
		return 0.3
	case roc >= -0.05:
		return 0.15
	default:
		return 0.0
	}
}

// volumeFraction — ponytail: MarketData (T2 fetcher) does not pull traded
// volume yet, so the volume-trend factor is scored neutral. Add a Volume field
// to MarketData and replace this with real logic (e.g. 20-bar avg vs latest).
func volumeFraction(md MarketData) float64 {
	_ = md
	return 0.5
}

func revGrowthFractionShort(md MarketData) float64 {
	switch {
	case md.RevGrowth >= 15:
		return 1.0
	case md.RevGrowth >= 10:
		return 0.8
	case md.RevGrowth >= 5:
		return 0.6
	case md.RevGrowth >= 0:
		return 0.4
	default:
		return 0.15
	}
}

func earningsMomentumFraction(md MarketData) float64 {
	return revGrowthFractionShort(md)*0.6 + netMarginFraction(md)*0.4
}

// scoreShort: Trend Teknis 30, Momentum 25, Volume 15, Valuasi 15,
// Earnings Momentum 15 (docs/domain.md).
func scoreShort(md MarketData) (int, map[string]int) {
	return assemble([]scoreComponent{
		{"trend_teknis", 30, trendTeknisFraction(md)},
		{"momentum", 25, momentumFraction(md)},
		{"volume", 15, volumeFraction(md)},
		{"valuasi", 15, valuasiFraction(md)},
		{"earnings_momentum", 15, earningsMomentumFraction(md)},
	})
}

// ---- Long-term (3-5 tahun): fundamentals-led ----

func roeFraction(md MarketData) float64 {
	switch {
	case md.ROE >= 20:
		return 1.0
	case md.ROE >= 15:
		return 0.85 // Balanced-Growth bonus band (consistent ROE > 15%)
	case md.ROE >= 8:
		return 0.6
	case md.ROE >= 0:
		return 0.3
	default:
		return 0.1
	}
}

func profitabilitasFraction(md MarketData) float64 {
	return roeFraction(md)*0.65 + netMarginFraction(md)*0.35
}

func solvabilitasFraction(md MarketData) float64 {
	switch {
	case md.DER <= 0.5:
		return 1.0
	case md.DER <= 1.0:
		return 0.85
	case md.DER <= 1.5:
		return 0.65
	case md.DER <= 2.0:
		return 0.4
	default:
		return 0.15 // Balanced-Growth penalty: DER > 2.0
	}
}

func pertumbuhanFraction(md MarketData) float64 {
	switch {
	case md.RevGrowth >= 15:
		return 1.0
	case md.RevGrowth >= 10:
		return 0.85 // Balanced-Growth bonus band (rev growth > 10%)
	case md.RevGrowth >= 5:
		return 0.6
	case md.RevGrowth >= 0:
		return 0.35
	default:
		return 0.1 // declining revenue penalized
	}
}

// trendLongFraction scores the price position vs MA200 (broad trend).
func trendLongFraction(md MarketData) float64 {
	if md.MA200 <= 0 {
		return 0.5 // insufficient history → neutral
	}
	switch r := md.LastPrice / md.MA200; {
	case r >= 1.1:
		return 1.0
	case r >= 1.0:
		return 0.7
	case r >= 0.95:
		return 0.4
	default:
		return 0.15
	}
}

// scoreLong: Profitabilitas 30, Solvabilitas 20, Valuasi 20, Pertumbuhan 15,
// Trend Teknis 15 (docs/domain.md).
func scoreLong(md MarketData) (int, map[string]int) {
	return assemble([]scoreComponent{
		{"profitabilitas", 30, profitabilitasFraction(md)},
		{"solvabilitas", 20, solvabilitasFraction(md)},
		{"valuasi", 20, valuasiFraction(md)},
		{"pertumbuhan", 15, pertumbuhanFraction(md)},
		{"trend_teknis", 15, trendLongFraction(md)},
	})
}

// ---- Risk flags (ticker-level; docs/domain.md Risk Flags table) ----

// computeRiskFlags emits the Balanced-Growth risk flags derivable from a
// MarketData snapshot. ROE==0 is treated as "missing" (not low), since the T2
// fetcher zeroes ROE when Yahoo omits it.
func computeRiskFlags(md MarketData) []string {
	var flags []string
	if md.DER > 2.0 {
		flags = append(flags, "high_debt")
	}
	if md.ROE != 0 && md.ROE < 8 {
		flags = append(flags, "low_profitability")
	}
	if md.PERatio > 25 || md.PBVRatio > 3.0 {
		flags = append(flags, "overvalued")
	}
	if md.MA50 > 0 && md.MA200 > 0 && md.LastPrice < md.MA50 && md.MA50 < md.MA200 {
		flags = append(flags, "downtrend")
	}
	return flags
}

// ---- Analyze + persist ----

// analyzeTicker computes both-horizon rule verdicts for one snapshot. Pure +
// deterministic (timestamps set but not asserted by callers). Used by tests and
// the persist path.
func analyzeTicker(md MarketData) (short, long AnalysisResult) {
	shortScore, shortBD := scoreShort(md)
	longScore, longBD := scoreLong(md)
	flagsJSON, _ := json.Marshal(computeRiskFlags(md))
	shortBDJSON, _ := json.Marshal(shortBD)
	longBDJSON, _ := json.Marshal(longBD)
	now := time.Now().UTC()
	short = AnalysisResult{
		Ticker: md.Ticker, Horizon: horizonShort,
		RuleVerdict: verdictFromScore(shortScore), RuleScore: shortScore,
		RuleBreakdown: string(shortBDJSON), RiskFlags: string(flagsJSON),
		UpdatedAt: now,
	}
	long = AnalysisResult{
		Ticker: md.Ticker, Horizon: horizonLong,
		RuleVerdict: verdictFromScore(longScore), RuleScore: longScore,
		RuleBreakdown: string(longBDJSON), RiskFlags: string(flagsJSON),
		UpdatedAt: now,
	}
	return short, long
}

// scoreAndPersist upserts both-horizon AnalysisResult rows for a ticker after
// its market data refreshes, so verdicts stay in sync with the latest snapshot.
// A store error is logged and skipped; it never aborts the refresh batch.
func scoreAndPersist(db *gorm.DB, md MarketData) {
	short, long := analyzeTicker(md)
	for _, r := range []*AnalysisResult{&short, &long} {
		var existing AnalysisResult
		err := db.Where("ticker = ? AND horizon = ?", r.Ticker, r.Horizon).First(&existing).Error
		switch {
		case err == nil:
			r.ID = existing.ID
			r.CreatedAt = existing.CreatedAt
		case errors.Is(err, gorm.ErrRecordNotFound):
			r.ID = uuid.NewString()
		default:
			log.Printf("scoring %s/%s: baca existing gagal: %v", r.Ticker, r.Horizon, err)
			continue
		}
		if err := db.Save(r).Error; err != nil {
			log.Printf("scoring %s/%s: simpan gagal: %v", r.Ticker, r.Horizon, err)
		}
	}
}

// ---- GET /api/v1/stocks/:ticker ----

type marketDataDTO struct {
	LastPrice float64 `json:"last_price"`
	PrevClose float64 `json:"prev_close"`
	PERatio   float64 `json:"pe_ratio"`
	PBVRatio  float64 `json:"pbv_ratio"`
	ROE       float64 `json:"roe"`
	DER       float64 `json:"der"`
	RevGrowth float64 `json:"rev_growth"`
	NetMargin float64 `json:"net_margin"`
	MA20      float64 `json:"ma20"`
	MA50      float64 `json:"ma50"`
	MA200     float64 `json:"ma200"`
	UpdatedAt string  `json:"updated_at"`
}

type ruleDTO struct {
	Verdict   string         `json:"verdict"`
	Score     int            `json:"score"`
	Breakdown map[string]int `json:"breakdown"`
}

// aiVerdictDTO is the (empty-until-T4) AI side of a horizon card. All fields
// stay zero-value/nil until the Hermes bridge populates them.
type aiVerdictDTO struct {
	Verdict     string   `json:"verdict,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	RiskFactors []string `json:"risk_factors,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

type horizonDTO struct {
	HorizonLabel string        `json:"horizon"`   // "6-12 Bulan" / "3-5 Tahun"
	Rule         *ruleDTO      `json:"rule"`      // null only if no data at all
	AI           *aiVerdictDTO `json:"ai"`        // null until T4 Hermes run
	Disagreement bool          `json:"disagreement"`
	RiskFlags    []string      `json:"risk_flags"`
}

type stockDetailResponse struct {
	Ticker      string        `json:"ticker"`
	CompanyName string        `json:"company_name"`
	MarketData  marketDataDTO `json:"market_data"`
	ShortTerm   horizonDTO    `json:"short_term"`
	LongTerm    horizonDTO    `json:"long_term"`
}

func toRuleDTO(ar AnalysisResult) ruleDTO {
	var bd map[string]int
	if ar.RuleBreakdown != "" {
		_ = json.Unmarshal([]byte(ar.RuleBreakdown), &bd)
	}
	if bd == nil {
		bd = map[string]int{}
	}
	return ruleDTO{Verdict: ar.RuleVerdict, Score: ar.RuleScore, Breakdown: bd}
}

func riskFlagsOf(ar AnalysisResult) []string {
	var f []string
	if ar.RiskFlags != "" && ar.RiskFlags != "null" {
		_ = json.Unmarshal([]byte(ar.RiskFlags), &f)
	}
	return f
}

// toAIDTO maps the persisted AI fields of an AnalysisResult onto the detail DTO.
// Returns nil when no AI run has happened yet (AIVerdict empty) so the frontend
// renders the "Analisis AI" trigger rather than an empty card.
func toAIDTO(ar AnalysisResult) *aiVerdictDTO {
	if ar.AIVerdict == "" {
		return nil
	}
	var risks []string
	if ar.AIRiskFactors != "" && ar.AIRiskFactors != "null" {
		_ = json.Unmarshal([]byte(ar.AIRiskFactors), &risks)
	}
	updated := ""
	if ar.AIUpdatedAt != nil && !ar.AIUpdatedAt.IsZero() {
		updated = ar.AIUpdatedAt.UTC().Format(time.RFC3339)
	}
	return &aiVerdictDTO{
		Verdict:     ar.AIVerdict,
		Explanation: ar.AIExplanation,
		Confidence:  ar.AIConfidence,
		RiskFactors: risks,
		UpdatedAt:   updated,
	}
}

// disagree is true when both rule and AI verdicts are present and differ.
func disagree(ruleVerdict, aiVerdict string) bool {
	return ruleVerdict != "" && aiVerdict != "" && ruleVerdict != aiVerdict
}

// loadAnalysis returns the persisted AnalysisResult for a ticker+horizon, or
// derives one from the live snapshot when no row exists yet (e.g. data fetched
// before scoring was wired). Keeps the detail view never-empty.
func loadAnalysis(db *gorm.DB, ticker, horizon string, md MarketData) AnalysisResult {
	var ar AnalysisResult
	if err := db.Where("ticker = ? AND horizon = ?", ticker, horizon).First(&ar).Error; err == nil {
		return ar
	}
	short, long := analyzeTicker(md)
	if horizon == horizonShort {
		return short
	}
	return long
}

func horizonLabel(horizon string) string {
	if horizon == horizonLong {
		return horizonLongLabel
	}
	return horizonShortLabel
}

// GET /api/v1/stocks/:ticker — market data + short/long rule+AI breakdown.
// ponytail: path follows docs/api-contract.md (`/stocks/:ticker`), not the
// `/portfolio/:id/detail` shape mentioned in the task brief — the contract and
// issue acceptance criteria are the source of truth.
func getStockDetail(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ticker := strings.ToUpper(strings.TrimSpace(c.Params("ticker")))
		resp, err := buildStockDetail(db, ticker)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "data pasar tidak ditemukan untuk ticker ini"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat data pasar"})
		}
		return c.JSON(resp)
	}
}

// buildStockDetail assembles the stock-detail response for a ticker. Shared by
// GET /stocks/:ticker and the T4 AI-analyze handler (which re-renders detail
// after storing a fresh AI verdict). Returns gorm.ErrRecordNotFound when no
// MarketData row exists.
func buildStockDetail(db *gorm.DB, ticker string) (stockDetailResponse, error) {
	var md MarketData
	if err := db.First(&md, "ticker = ?", ticker).Error; err != nil {
		return stockDetailResponse{}, err
	}

	shortAR := loadAnalysis(db, ticker, horizonShort, md)
	longAR := loadAnalysis(db, ticker, horizonLong, md)
	shortRule := toRuleDTO(shortAR)
	longRule := toRuleDTO(longAR)
	updated := ""
	if !md.UpdatedAt.IsZero() {
		updated = md.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return stockDetailResponse{
		Ticker:      md.Ticker,
		CompanyName: md.CompanyName,
		MarketData: marketDataDTO{
			LastPrice: md.LastPrice,
			PrevClose: md.PrevClose,
			PERatio:   md.PERatio,
			PBVRatio:  md.PBVRatio,
			ROE:       md.ROE,
			DER:       md.DER,
			RevGrowth: md.RevGrowth,
			NetMargin: md.NetMargin,
			MA20:      md.MA20,
			MA50:      md.MA50,
			MA200:     md.MA200,
			UpdatedAt: updated,
		},
		ShortTerm: horizonDTO{
			HorizonLabel: horizonLabel(horizonShort),
			Rule:         &shortRule,
			AI:           toAIDTO(shortAR),
			Disagreement: disagree(shortAR.RuleVerdict, shortAR.AIVerdict),
			RiskFlags:    riskFlagsOf(shortAR),
		},
		LongTerm: horizonDTO{
			HorizonLabel: horizonLabel(horizonLong),
			Rule:         &longRule,
			AI:           toAIDTO(longAR),
			Disagreement: disagree(longAR.RuleVerdict, longAR.AIVerdict),
			RiskFlags:    riskFlagsOf(longAR),
		},
	}, nil
}
