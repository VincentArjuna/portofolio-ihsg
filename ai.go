package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ai.go — the Hermes CLI subprocess bridge (T4).
//
// Flow: build a context JSON (ticker, portfolio weight, delayed market data +
// technicals, rule scores, sources) → invoke `hermes chat -q <prompt> -Q` via
// os/exec → parse the structured JSON verdict → upsert AI fields onto the
// short/long AnalysisResult rows. Results are cached for 24h (ai_updated_at);
// when Hermes is absent or returns garbage the endpoint degrades gracefully
// (status: unavailable|error) without crashing.

// 24h cache window matches the daily refresh cadence (PRD); a fresh AI run is
// reused instead of re-invoking the (slow, paid) Hermes CLI.
const aiCacheTTL = 24 * time.Hour

// hermesTimeout bounds the synchronous subprocess call. Hermes can take a while
// on a cold model; the single-user local client waits up to this long.
const hermesTimeout = 150 * time.Second

var errHermesUnavailable = errors.New("hermes tidak tersedia")

// --- Context built for Hermes (docs/architecture.md §4 shape) ---

type aiPortfolioCtx struct {
	Shares    int     `json:"shares"`
	WeightPct float64 `json:"weight_pct"`
}

type aiMarketCtx struct {
	Price     float64 `json:"price"`
	PER       float64 `json:"per"`
	PBV       float64 `json:"pbv"`
	ROE       float64 `json:"roe"`
	DER       float64 `json:"der"`
	RevGrowth float64 `json:"rev_growth"`
	NetMargin float64 `json:"net_margin"`
}

type aiTechnicalCtx struct {
	MA20  float64 `json:"ma20"`
	MA50  float64 `json:"ma50"`
	MA200 float64 `json:"ma200"`
}

type aiRuleSide struct {
	Verdict string `json:"verdict"`
	Score   int    `json:"score"`
}

type aiContext struct {
	Ticker     string         `json:"ticker"`
	Portfolio  aiPortfolioCtx `json:"portfolio_context"`
	MarketData aiMarketCtx    `json:"market_data"`
	Technicals aiTechnicalCtx `json:"technical_indicators"`
	RuleScore  struct {
		Short aiRuleSide `json:"short"`
		Long  aiRuleSide `json:"long"`
	} `json:"rule_score"`
	Sources []string `json:"sources"`
}

// buildAIContext assembles the structured snapshot handed to Hermes. Weight is
// cost-basis allocation (stable; avoids pulling every ticker's market data).
// Returns gorm.ErrRecordNotFound when the ticker has no market data yet.
func buildAIContext(db *gorm.DB, ticker string) (aiContext, error) {
	var md MarketData
	if err := db.First(&md, "ticker = ?", ticker).Error; err != nil {
		return aiContext{}, err
	}

	var positions []Position
	_ = db.Find(&positions).Error
	totalCost, thisCost, shares := 0.0, 0.0, 0
	for _, p := range positions {
		c := float64(p.Shares) * p.AvgBuyPrice
		totalCost += c
		if p.Ticker == ticker {
			thisCost, shares = c, p.Shares
		}
	}
	weightPct := 0.0
	if totalCost > 0 {
		weightPct = round1(thisCost / totalCost * 100)
	}

	sources := []string{yahooQuoteURL(ticker), "https://www.idx.co.id/"}
	if md.SourceURL != "" {
		sources[0] = md.SourceURL
	}

	shortAR := loadAnalysis(db, ticker, horizonShort, md)
	longAR := loadAnalysis(db, ticker, horizonLong, md)

	ctx := aiContext{
		Ticker:     ticker,
		Portfolio:  aiPortfolioCtx{Shares: shares, WeightPct: weightPct},
		MarketData: aiMarketCtx{Price: md.LastPrice, PER: md.PERatio, PBV: md.PBVRatio, ROE: md.ROE, DER: md.DER, RevGrowth: md.RevGrowth, NetMargin: md.NetMargin},
		Technicals: aiTechnicalCtx{MA20: md.MA20, MA50: md.MA50, MA200: md.MA200},
		Sources:    sources,
	}
	ctx.RuleScore.Short = aiRuleSide{Verdict: shortAR.RuleVerdict, Score: shortAR.RuleScore}
	ctx.RuleScore.Long = aiRuleSide{Verdict: longAR.RuleVerdict, Score: longAR.RuleScore}
	return ctx, nil
}

// renderPrompt embeds the context JSON and demands a strict JSON schema in
// Indonesian. "HANYA JSON" + the explicit schema keeps parsing reliable.
func renderPrompt(c aiContext) string {
	b, _ := json.Marshal(c)
	return `Kamu adalah analis saham IHSG independen untuk profil risiko Balanced-Growth.
Beri rekomendasi HANYA berdasarkan konteks JSON di bawah — jangan mengambil atau mengasumsikan data eksternal.
Semua teks (reasoning, risk_factors, data_limitations) WAJIB dalam Bahasa Indonesia.
Output HANYA JSON valid (tanpa markdown, tanpa teks di luar JSON) dengan schema persis:

{"short_term":{"verdict":"BUY|HOLD|SELL","confidence":0.0,"reasoning":"alasan jangka pendek 6-12 bulan"},
 "long_term":{"verdict":"BUY|HOLD|SELL","confidence":0.0,"reasoning":"alasan jangka panjang 3-5 tahun"},
 "risk_factors":["faktor risiko"],
 "data_limitations":["keterbatasan data tertunda/missing"]}

confidence antara 0.0 dan 1.0. Konteks:
` + string(b)
}

// runHermes invokes `hermesPath chat -q <prompt> -Q` and returns stdout. -Q is
// the programmatic/quiet flag (final response + session line only, no banner).
func runHermes(ctx context.Context, hermesPath, prompt string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, hermesPath, "chat", "-q", prompt, "-Q")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("hermes: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// --- Parsed Hermes output ---

type aiHorizonResult struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

type aiResult struct {
	ShortTerm       aiHorizonResult `json:"short_term"`
	LongTerm        aiHorizonResult `json:"long_term"`
	RiskFactors     []string        `json:"risk_factors"`
	DataLimitations []string        `json:"data_limitations"`
}

// parseAIResult extracts the JSON object from Hermes stdout and validates it.
// Hermes -Q appends a trailing `session_id: ...` line (no braces) and some
// models wrap JSON in markdown fences; bounding by first '{' and last '}'
// handles both without a fragile line-by-line scan.
func parseAIResult(stdout []byte) (aiResult, error) {
	var r aiResult
	s := string(stdout)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return r, errors.New("tidak ada objek JSON dalam respons Hermes")
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &r); err != nil {
		return r, fmt.Errorf("gagal parse JSON Hermes: %w", err)
	}
	r.ShortTerm.Verdict = canonVerdict(r.ShortTerm.Verdict)
	r.LongTerm.Verdict = canonVerdict(r.LongTerm.Verdict)
	if r.ShortTerm.Verdict == "" || r.LongTerm.Verdict == "" {
		return r, errors.New("verdict Hermes kosong/tidak dikenal")
	}
	return r, nil
}

// canonVerdict normalizes free-form verdict text to the canonical BUY/HOLD/SELL
// (accepts English + Indonesian: Beli/Tahan/Jual, "Strong Buy", etc.).
func canonVerdict(v string) string {
	l := strings.ToLower(strings.TrimSpace(v))
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "buy") || strings.Contains(l, "beli"):
		return verdictBuy
	case strings.Contains(l, "sell") || strings.Contains(l, "jual"):
		return verdictSell
	case strings.Contains(l, "hold") || strings.Contains(l, "tahan") || strings.Contains(l, "keep"):
		return verdictHold
	default:
		return ""
	}
}

// persistAIResult upserts AI fields onto the short/long AnalysisResult rows for
// a ticker. Existing rows (the normal case — created at refresh/scoring time)
// are updated in place; missing rows are derived from the live snapshot so the
// table never ends up half-populated. data_limitations are folded into the
// explanation (domain.md has no separate limitations column).
func persistAIResult(db *gorm.DB, ticker string, res aiResult, now time.Time) error {
	var md MarketData
	hasMD := db.First(&md, "ticker = ?", ticker).Error == nil

	risksJSON, _ := json.Marshal(res.RiskFactors)
	limitations := ""
	if len(res.DataLimitations) > 0 {
		limitations = "\n\nKeterbatasan data: " + strings.Join(res.DataLimitations, "; ")
	}

	sides := []struct {
		horizon, verdict, reasoning string
		confidence                  float64
	}{
		{horizonShort, res.ShortTerm.Verdict, res.ShortTerm.Reasoning, res.ShortTerm.Confidence},
		{horizonLong, res.LongTerm.Verdict, res.LongTerm.Reasoning, res.LongTerm.Confidence},
	}
	for _, s := range sides {
		var ar AnalysisResult
		err := db.Where("ticker = ? AND horizon = ?", ticker, s.horizon).First(&ar).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if !hasMD {
				continue // nothing to derive from; skip rather than store a rule-less row
			}
			short, long := analyzeTicker(md)
			if s.horizon == horizonShort {
				ar = short
			} else {
				ar = long
			}
			ar.ID = uuid.NewString()
		case err != nil:
			return err
		}
		ar.AIVerdict = s.verdict
		ar.AIExplanation = strings.TrimSpace(s.reasoning) + limitations
		ar.AIConfidence = s.confidence
		ar.AIRiskFactors = string(risksJSON)
		t := now
		ar.AIUpdatedAt = &t
		if err := db.Save(&ar).Error; err != nil {
			return err
		}
	}
	return nil
}

// aiCacheFresh is true when any horizon row for the ticker carries an AI run
// newer than aiCacheTTL — reuse it instead of re-invoking Hermes.
func aiCacheFresh(db *gorm.DB, ticker string) (bool, error) {
	var rows []AnalysisResult
	if err := db.Where("ticker = ? AND ai_updated_at IS NOT NULL", ticker).Find(&rows).Error; err != nil {
		return false, err
	}
	for _, r := range rows {
		if r.AIUpdatedAt != nil && !r.AIUpdatedAt.IsZero() && time.Since(*r.AIUpdatedAt) < aiCacheTTL {
			return true, nil
		}
	}
	return false, nil
}

// --- HTTP ---

type aiAnalyzeResponse struct {
	Status  string               `json:"status"` // done | cached | unavailable | error
	Message string               `json:"message,omitempty"`
	Detail  *stockDetailResponse `json:"detail,omitempty"`
}

// POST /api/v1/stocks/:ticker/ai-analyze — runs Hermes AI analysis for one held
// ticker and returns the updated stock detail (rule + AI side-by-side).
//
// ponytail: the api-contract/issue describe a 202 + async job, but this is a
// single-user local app with no job queue — the call is synchronous and returns
// the result directly (200). The frontend's "Hermes sedang menganalisis..."
// spinner covers the wait. Swap in a goroutine + polling if latency grows.
func analyzeStockAI(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ticker := strings.ToUpper(strings.TrimSpace(c.Params("ticker")))

		// 404 fast if the ticker has no market data to analyze.
		if _, err := buildAIContext(db, ticker); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(404).JSON(fiber.Map{"error": "ticker belum memiliki data pasar"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat konteks"})
		}

		// 24h cache: hand back the stored detail without touching Hermes.
		if fresh, err := aiCacheFresh(db, ticker); err == nil && fresh {
			detail, _ := buildStockDetail(db, ticker)
			return c.JSON(aiAnalyzeResponse{Status: "cached", Detail: &detail})
		}

		s, err := loadSettings(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat pengaturan"})
		}
		hermesPath := s.HermesExecutable
		if hermesPath == "" {
			hermesPath = "hermes"
		}
		if _, lerr := exec.LookPath(hermesPath); lerr != nil {
			return c.JSON(aiAnalyzeResponse{
				Status:  "unavailable",
				Message: fmt.Sprintf(`Hermes AI tidak tersedia (executable "%s" tidak ditemukan di PATH). Pasang Hermes atau atur path-nya di Pengaturan.`, hermesPath),
			})
		}

		ctxJSON, _ := buildAIContext(db, ticker)
		prompt := renderPrompt(ctxJSON)

		runCtx, cancel := context.WithTimeout(c.Context(), hermesTimeout)
		defer cancel()
		out, runErr := runHermes(runCtx, hermesPath, prompt)
		if runErr != nil {
			log.Printf("ai %s: hermes gagal: %v", ticker, runErr)
			return c.JSON(aiAnalyzeResponse{Status: "error", Message: "Hermes gagal menjalankan analisis. Coba lagi nanti."})
		}
		res, perr := parseAIResult(out)
		if perr != nil {
			log.Printf("ai %s: parse gagal: %v | raw: %q", ticker, perr, truncate(string(out), 200))
			return c.JSON(aiAnalyzeResponse{Status: "error", Message: "Hermes mengembalikan respons yang tidak bisa diurai."})
		}
		if err := persistAIResult(db, ticker, res, time.Now().UTC()); err != nil {
			log.Printf("ai %s: simpan gagal: %v", ticker, err)
			return c.Status(500).JSON(fiber.Map{"error": "gagal menyimpan hasil AI"})
		}
		detail, _ := buildStockDetail(db, ticker)
		return c.JSON(aiAnalyzeResponse{Status: "done", Detail: &detail})
	}
}

// round1 rounds to one decimal place (weight_pct presentation).
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// truncate caps s at n runes for safe logging of Hermes output.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
