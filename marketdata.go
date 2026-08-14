package main

import (
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// MarketData is the delayed quote + fundamental snapshot for one IHSG ticker.
// Keyed by ticker (per docs/domain.md). Price/MA fields come from the Yahoo
// Finance chart endpoint; ratio fields are best-effort from quoteSummary.
type MarketData struct {
	Ticker      string    `gorm:"primaryKey" json:"ticker"`
	CompanyName string    `json:"company_name"`
	LastPrice   float64   `json:"last_price"`
	PrevClose   float64   `json:"prev_close"`
	PERatio     float64   `json:"pe_ratio"`
	PBVRatio    float64   `json:"pbv_ratio"`
	ROE         float64   `json:"roe"`
	DER         float64   `json:"der"`
	RevGrowth   float64   `json:"rev_growth"`
	NetMargin   float64   `json:"net_margin"`
	MA20        float64   `json:"ma20"`
	MA50        float64   `json:"ma50"`
	MA200       float64   `json:"ma200"`
	SourceURL   string    `json:"source_url"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const (
	yahooUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36"
	staleAfter     = 24 * time.Hour // PRD: daily refresh; older data is "stale"
)

// newHTTPClient returns a client with a cookie jar (needed for Yahoo crumb flow).
func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
	}
}

func httpGet(client *http.Client, url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// --- Yahoo Finance chart endpoint (price + history; no crumb required) ---

type chartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
				LongName             string  `json:"longName"`
				ShortName            string  `json:"shortName"`
				RegularMarketTime    int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Timestamp []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"` // nullable per session
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// parseChartBody extracts price fields, company name, and MA20/50/200 from a
// Yahoo chart payload. last_price is the intraday regularMarketPrice; prev_close
// is the most recent completed session's close (one bar before the live price).
func parseChartBody(body []byte, ticker string) (MarketData, error) {
	var cr chartResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return MarketData{}, err
	}
	if cr.Chart.Error != nil {
		return MarketData{}, &chartError{msg: cr.Chart.Error.Description}
	}
	if len(cr.Chart.Result) == 0 {
		return MarketData{}, &chartError{msg: "chart kosong"}
	}
	r := cr.Chart.Result[0]
	md := MarketData{
		Ticker:    ticker,
		LastPrice: r.Meta.RegularMarketPrice,
	}
	md.CompanyName = r.Meta.LongName
	if md.CompanyName == "" {
		md.CompanyName = r.Meta.ShortName
	}

	// Flatten non-nil closes in chronological order.
	var closes []float64
	if len(r.Indicators.Quote) > 0 {
		for _, p := range r.Indicators.Quote[0].Close {
			if p != nil {
				closes = append(closes, *p)
			}
		}
	}

	// prev_close = last completed session. If the final bar equals the live
	// intraday price, the market is closed and that bar is "today" → use the
	// prior bar. Fallback to chartPreviousClose when history is thin.
	switch {
	case len(closes) >= 2 && math.Abs(closes[len(closes)-1]-md.LastPrice) < 1e-6:
		md.PrevClose = closes[len(closes)-2]
	case len(closes) >= 1:
		md.PrevClose = closes[len(closes)-1]
	default:
		md.PrevClose = r.Meta.ChartPreviousClose
	}

	md.MA20 = ma(closes, 20)
	md.MA50 = ma(closes, 50)
	md.MA200 = ma(closes, 200)
	md.SourceURL = yahooQuoteURL(ticker)
	return md, nil
}

type chartError struct{ msg string }

func (e *chartError) Error() string { return e.msg }

// ma returns the arithmetic mean of the last `period` closes, or 0 when there
// is insufficient history for that window.
func ma(closes []float64, period int) float64 {
	if len(closes) < period {
		return 0
	}
	sum := 0.0
	for _, v := range closes[len(closes)-period:] {
		sum += v
	}
	return sum / float64(period)
}

func fetchChart(client *http.Client, ticker string) (MarketData, error) {
	url := "https://query1.finance.yahoo.com/v8/finance/chart/" +
		ticker + ".JK?range=1y&interval=1d"
	body, status, err := httpGet(client, url)
	if err != nil {
		return MarketData{}, err
	}
	if status != 200 {
		return MarketData{}, &chartError{msg: "chart HTTP " + strconv.Itoa(status)}
	}
	return parseChartBody(body, ticker)
}

// --- Yahoo Finance quoteSummary endpoint (fundamentals; needs crumb + cookie) ---

type quoteSummaryResponse struct {
	QuoteSummary struct {
		Result []struct {
			SummaryDetail struct {
				TrailingPE *struct{ Raw float64 `json:"raw"` } `json:"trailingPE"`
			} `json:"summaryDetail"`
			DefaultKeyStatistics struct {
				ForwardPE  *struct{ Raw float64 `json:"raw"` } `json:"forwardPE"`
				PriceToBook *struct{ Raw float64 `json:"raw"` } `json:"priceToBook"`
			} `json:"defaultKeyStatistics"`
			FinancialData struct {
				ReturnOnEquity *struct{ Raw float64 `json:"raw"` } `json:"returnOnEquity"`
				DebtToEquity   *struct{ Raw float64 `json:"raw"` } `json:"debtToEquity"`
				RevenueGrowth  *struct{ Raw float64 `json:"raw"` } `json:"revenueGrowth"`
				ProfitMargins  *struct{ Raw float64 `json:"raw"` } `json:"profitMargins"`
			} `json:"financialData"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"quoteSummary"`
}

// parseQuoteSummaryBody fills the ratio fields of md from a Yahoo quoteSummary
// payload. Yahoo stores ratios as fractions (0.18) or percent-scaled (debt/equity
// 23.6); ROE/rev/margin are reported as percentages, DER as a ratio.
func parseQuoteSummaryBody(body []byte, md *MarketData) error {
	var qs quoteSummaryResponse
	if err := json.Unmarshal(body, &qs); err != nil {
		return err
	}
	if qs.QuoteSummary.Error != nil {
		return &chartError{msg: qs.QuoteSummary.Error.Description}
	}
	if len(qs.QuoteSummary.Result) == 0 {
		return &chartError{msg: "quoteSummary kosong"}
	}
	r := qs.QuoteSummary.Result[0]
	if r.SummaryDetail.TrailingPE != nil {
		md.PERatio = r.SummaryDetail.TrailingPE.Raw
	} else if r.DefaultKeyStatistics.ForwardPE != nil {
		md.PERatio = r.DefaultKeyStatistics.ForwardPE.Raw
	}
	if r.DefaultKeyStatistics.PriceToBook != nil {
		md.PBVRatio = r.DefaultKeyStatistics.PriceToBook.Raw
	}
	if r.FinancialData.ReturnOnEquity != nil {
		md.ROE = r.FinancialData.ReturnOnEquity.Raw * 100
	}
	if r.FinancialData.DebtToEquity != nil {
		md.DER = r.FinancialData.DebtToEquity.Raw / 100 // Yahoo reports percent-scaled
	}
	if r.FinancialData.RevenueGrowth != nil {
		md.RevGrowth = r.FinancialData.RevenueGrowth.Raw * 100
	}
	if r.FinancialData.ProfitMargins != nil {
		md.NetMargin = r.FinancialData.ProfitMargins.Raw * 100
	}
	return nil
}

// fetchFundamentals attempts the Yahoo crumb/cookie flow for ratio data. It
// fails often (Yahoo rate-limits / requires consent); callers degrade to zeros.
func fetchFundamentals(client *http.Client, ticker string, md *MarketData) error {
	if _, _, err := httpGet(client, "https://finance.yahoo.com/quote/"+ticker+".JK/"); err != nil {
		return err
	}
	crumbBody, status, err := httpGet(client, "https://query2.finance.yahoo.com/v1/test/getcrumb")
	if err != nil {
		return err
	}
	if status != 200 {
		return &chartError{msg: "getcrumb HTTP " + strconv.Itoa(status)}
	}
	crumb := strings.TrimSpace(string(crumbBody))
	url := "https://query2.finance.yahoo.com/v10/finance/quoteSummary/" +
		ticker + ".JK?modules=summaryDetail,defaultKeyStatistics,financialData&crumb=" + crumb
	body, status, err := httpGet(client, url)
	if err != nil {
		return err
	}
	if status != 200 {
		return &chartError{msg: "quoteSummary HTTP " + strconv.Itoa(status)}
	}
	return parseQuoteSummaryBody(body, md)
}

func yahooQuoteURL(ticker string) string {
	return "https://finance.yahoo.com/quote/" + ticker + ".JK"
}

// fetchMarketData pulls price (critical) then fundamentals (best-effort).
// Returns an error only when price fetching fails — a fundamentals failure
// leaves the ratio fields at zero so a single ticker still gets stored.
func fetchMarketData(ticker string) (MarketData, error) {
	client := newHTTPClient()
	md, err := fetchChart(client, ticker)
	if err != nil {
		return MarketData{}, err
	}
	if ferr := fetchFundamentals(client, ticker, &md); ferr != nil {
		// Not fatal: keep price + MAs, zero the ratios. Logged for visibility.
		log.Printf("market-data %s: fundamental dilewati: %v", ticker, ferr)
	}
	return md, nil
}

// fetchMarketDataFn is the market-data fetch seam. It defaults to the real
// Yahoo fetcher; tests override it with a deterministic stub so the full
// refresh → score → P&L pipeline can be exercised hermetically (no network).
//
// The background auto-analyze goroutine (issue #17) reads it concurrently with
// test swaps, so reads/writes go through currentFetcher/setFetcher under a
// RWMutex — a bare global read would race the test cleanup that restores it.
var (
	fetchMarketDataMu sync.RWMutex
	fetchMarketDataFn = fetchMarketData
)

func currentFetcher() func(string) (MarketData, error) {
	fetchMarketDataMu.RLock()
	defer fetchMarketDataMu.RUnlock()
	return fetchMarketDataFn
}

func setFetcher(fn func(string) (MarketData, error)) {
	fetchMarketDataMu.Lock()
	defer fetchMarketDataMu.Unlock()
	fetchMarketDataFn = fn
}

// --- Refresh core (shared by manual handler + background scheduler + T5 universe) ---

// refreshTickers fetches + stores market data and re-scores a deduped ticker
// set. A single ticker failure is logged and skipped; it never aborts the
// batch. lastUpdate is the newest UpdatedAt stored, or zero when nothing
// refreshed. Shared by the held-ticker refresh and the opportunity-universe
// refresh (T5).
func refreshTickers(db *gorm.DB, tickers []string) (refreshed, failed int, lastUpdate time.Time) {
	fetch := currentFetcher() // snapshot once; the seam may be swapped by tests
	seen := make(map[string]struct{}, len(tickers))
	for _, t := range tickers {
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}

		md, ferr := fetch(t)
		if ferr != nil {
			log.Printf("refresh %s gagal: %v", t, ferr)
			failed++
			continue
		}
		md.UpdatedAt = time.Now().UTC()
		if serr := db.Save(&md).Error; serr != nil {
			log.Printf("refresh %s simpan gagal: %v", t, serr)
			failed++
			continue
		}
		// Re-score on every refresh so verdicts track the latest snapshot.
		scoreAndPersist(db, md)
		if md.UpdatedAt.After(lastUpdate) {
			lastUpdate = md.UpdatedAt
		}
		refreshed++
	}
	return refreshed, failed, lastUpdate
}

// refreshSet is the deduped union of held tickers + the LQ45/Kompas100 universe.
// The background/manual refresh covers the whole liquid universe, not just held
// positions (issue #6: market-data refresh extended to cover the universe).
func refreshSet(db *gorm.DB) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(t string) {
		if t == "" {
			return
		}
		if _, dup := seen[t]; dup {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}

	var positions []Position
	if err := db.Find(&positions).Error; err != nil {
		return nil, err
	}
	for _, p := range positions {
		add(p.Ticker)
	}

	var opps []Opportunity
	if err := db.Find(&opps).Error; err == nil { // tolerant: empty/unseeded → skip
		for _, o := range opps {
			add(o.Ticker)
		}
	}
	return out, nil
}

// runRefresh fetches + stores market data for every held ticker and every
// universe ticker, deduped. err is non-nil only when the position read fails
// (callers surface that as 500 / retry-next-slot).
func runRefresh(db *gorm.DB) (refreshed, failed int, lastUpdate time.Time, err error) {
	tickers, err := refreshSet(db)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	refreshed, failed, lastUpdate = refreshTickers(db, tickers)
	return refreshed, failed, lastUpdate, nil
}

// --- HTTP handler ---

type refreshResponse struct {
	RefreshedCount int    `json:"refreshed_count"`
	UpdatedAt      string `json:"updated_at"`
	Failed         int    `json:"failed"`
}

// POST /api/v1/market-data/refresh — fetch delayed data for every held ticker.
// A single ticker failure is logged and skipped; it never aborts the batch.
func refreshMarketData(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		refreshed, failed, lastUpdate, err := runRefresh(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal membaca portofolio"})
		}
		updated := ""
		if !lastUpdate.IsZero() {
			updated = lastUpdate.Format(time.RFC3339)
		}
		return c.JSON(refreshResponse{
			RefreshedCount: refreshed,
			UpdatedAt:      updated,
			Failed:         failed,
		})
	}
}
