package main

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Index-membership tokens (docs/domain.md "illiquid" flag: anything outside
// LQ45/Kompas100 is not a liquid candidate).
const (
	idxLQ45   = "LQ45"
	idxKompas = "KOMPAS100"
	idxBoth   = "BOTH"
)

// Opportunity is one row of the LQ45/Kompas100 liquid universe: ticker, the
// index(es) it belongs to, company name, and IDX sector. Price/fundamentals and
// rule scores live on MarketData/AnalysisResult (keyed by ticker) and are joined
// at read time — this table only records universe membership + classification.
type Opportunity struct {
	Ticker          string    `gorm:"primaryKey" json:"ticker"`
	IndexMembership string    `gorm:"not null" json:"index_membership"` // LQ45 | KOMPAS100 | BOTH
	Name            string    `json:"name"`
	Sector          string    `json:"sector"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// indexMembership maps the raw lq45/kompas flags to the stored token.
func indexMembership(lq45, kompas bool) string {
	switch {
	case lq45 && kompas:
		return idxBoth
	case lq45:
		return idxLQ45
	case kompas:
		return idxKompas
	}
	return ""
}

// seedUniverse is a cached snapshot of LQ45/Kompas100 constituents. IDX revises
// these lists twice a year and the official site renders constituents via JS, so
// a static snapshot is the pragmatic source for a local single-user app.
// ponytail: refresh by fetching https://www.idx.co.id listings and re-seeding;
// membership flags below are best-effort and will drift until then.
var seedUniverse = []struct {
	ticker, name, sector string
	lq45, kompas         bool
}{
	{"AALI", "PT Astra Agro Lestari Tbk", "Material Dasar", true, false},
	{"ACES", "PT Ace Hardware Indonesia Tbk", "Barang & Jasa Konsumen", false, true},
	{"ADRO", "PT Adaro Energy Indonesia Tbk", "Energi", true, true},
	{"AKRA", "PT AKR Corporindo Tbk", "Aneka Industri", true, true},
	{"AMMN", "PT Amman Mineral Nusantara Tbk", "Material Dasar", true, true},
	{"AMRT", "PT Sumber Alfaria Trijaya Tbk", "Barang Konsumen Primer", true, true},
	{"ANTM", "PT Aneka Tambang Tbk", "Material Dasar", true, true},
	{"ASII", "PT Astra International Tbk", "Aneka Industri", true, true},
	{"BBCA", "PT Bank Central Asia Tbk", "Keuangan", true, true},
	{"BBNI", "PT Bank Negara Indonesia Tbk", "Keuangan", true, true},
	{"BBRI", "PT Bank Rakyat Indonesia Tbk", "Keuangan", true, true},
	{"BBTN", "PT Bank Tabungan Negara Tbk", "Keuangan", false, true},
	{"BFIN", "PT BFI Finance Indonesia Tbk", "Keuangan", false, true},
	{"BMRI", "PT Bank Mandiri Tbk", "Keuangan", true, true},
	{"BSDE", "PT Bumi Serpong Damai Tbk", "Properti", true, true},
	{"BTPN", "PT Bank Tabungan Pensiunan Nasional Tbk", "Keuangan", false, true},
	{"CPIN", "PT Charoen Pokphand Indonesia Tbk", "Barang Konsumen Primer", true, true},
	{"CTRA", "PT Ciputra Development Tbk", "Properti", true, false},
	{"DCII", "PT DCI Indonesia Tbk", "Infrastruktur", false, true},
	{"EMTK", "PT Elang Mahkota Teknologi Tbk", "Teknologi", false, true},
	{"ESSA", "PT Surya Esa Perkasa Tbk", "Energi", true, false},
	{"EXCL", "PT XL Axiata Tbk", "Infrastruktur", true, false},
	{"GGRM", "PT Gudang Garam Tbk", "Barang Konsumen Primer", true, true},
	{"GOTO", "PT GoTo Gojek Tokopedia Tbk", "Teknologi", true, true},
	{"HRUM", "PT Harum Energy Tbk", "Energi", false, true},
	{"ICBP", "PT Indofood CBP Sukses Makmur Tbk", "Barang Konsumen Primer", true, true},
	{"INCO", "PT Vale Indonesia Tbk", "Material Dasar", true, true},
	{"INDF", "PT Indofood Sukses Makmur Tbk", "Barang Konsumen Primer", true, true},
	{"INKP", "PT Indah Kiat Pulp & Paper Tbk", "Material Dasar", true, false},
	{"INTP", "PT Indocement Tunggal Prakarsa Tbk", "Material Dasar", true, true},
	{"ISAT", "PT Indosat Tbk", "Infrastruktur", true, true},
	{"ITMG", "PT Indo Tambangraya Megah Tbk", "Energi", true, true},
	{"JPFA", "PT Japfa Komfeed Indonesia Tbk", "Barang Konsumen Primer", true, true},
	{"JSMR", "PT Jasa Marga Tbk", "Infrastruktur", false, true},
	{"KAEF", "PT Kimia Farma Tbk", "Kesehatan", false, true},
	{"KLBF", "PT Kalbe Farma Tbk", "Kesehatan", true, true},
	{"MAPI", "PT MAP Aktif Adiperkasa Tbk", "Barang & Jasa Konsumen", true, false},
	{"MAPA", "PT Mitra Adiperkasa Tbk", "Barang & Jasa Konsumen", false, true},
	{"MDKA", "PT Merdeka Copper Gold Tbk", "Material Dasar", true, true},
	{"MEDC", "PT Medco Energi Internasional Tbk", "Energi", false, true},
	{"MYOR", "PT Mayora Indah Tbk", "Barang Konsumen Primer", false, true},
	{"PGAS", "PT Perusahaan Gas Negara Tbk", "Energi", true, true},
	{"PTBA", "PT Bukit Asam Tbk", "Energi", true, true},
	{"PTPP", "PT Pembangunan Perumahan Tbk", "Aneka Industri", false, true},
	{"PWON", "PT Pakuwon Jati Tbk", "Properti", false, true},
	{"RALS", "PT Ramayana Lestari Sentosa Tbk", "Barang & Jasa Konsumen", false, true},
	{"SIDO", "PT Industri Jamu & Farmasi Sido Muncul Tbk", "Kesehatan", true, false},
	{"SMRA", "PT Summarecon Agung Tbk", "Properti", true, true},
	{"TINS", "PT Timah Tbk", "Material Dasar", false, true},
	{"TLKM", "PT Telkom Indonesia Tbk", "Infrastruktur", true, true},
	{"TOWR", "PT Sarana Menara Nusantara Tbk", "Infrastruktur", true, true},
	{"UNTR", "PT United Tractors Tbk", "Aneka Industri", true, true},
	{"UNVR", "PT Unilever Indonesia Tbk", "Barang Konsumen Primer", true, true},
	{"WSKT", "PT Wijaya Karya Tbk", "Aneka Industri", false, true},
}

// seedOpportunities upserts the cached universe into the Opportunity table.
// Idempotent: re-runs on every boot, refreshing membership/name/sector while
// preserving created_at. Safe to extend the list later.
func seedOpportunities(db *gorm.DB) {
	for _, s := range seedUniverse {
		opp := Opportunity{
			Ticker:          s.ticker,
			Name:            s.name,
			Sector:          s.sector,
			IndexMembership: indexMembership(s.lq45, s.kompas),
		}
		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ticker"}},
			DoUpdates: clause.AssignmentColumns([]string{"index_membership", "name", "sector", "updated_at"}),
		}).Create(&opp).Error
		if err != nil {
			log.Printf("seed opportunity %s: %v", s.ticker, err)
		}
	}
}

// ---- Pure filtering/ranking helpers (unit-tested) ----

// inIndex reports whether a stock of `membership` belongs to the requested
// filter. "lq45" matches LQ45 + BOTH; "kompas100" matches KOMPAS100 + BOTH;
// "both" matches only BOTH; ""/"all" matches everything.
func inIndex(membership, filter string) bool {
	switch strings.ToUpper(strings.TrimSpace(filter)) {
	case "", "ALL":
		return true
	case "LQ45":
		return membership == idxLQ45 || membership == idxBoth
	case "KOMPAS100", "KOMPAS":
		return membership == idxKompas || membership == idxBoth
	case "BOTH":
		return membership == idxBoth
	}
	return true
}

// verdictRank orders BUY > HOLD > SELL > (unscored). Used for Buy-first ranking.
func verdictRank(v string) int {
	switch v {
	case verdictBuy:
		return 2
	case verdictHold:
		return 1
	case verdictSell:
		return 0
	}
	return -1
}

// meetsMinVerdict reports whether `verdict` is at least `min` (BUY > HOLD > SELL).
// An empty/unknown `min` applies no filter.
func meetsMinVerdict(verdict, min string) bool {
	min = strings.ToUpper(strings.TrimSpace(min))
	if min == "" {
		return true
	}
	if min != verdictBuy && min != verdictHold && min != verdictSell {
		return true // unknown filter → don't hide everything
	}
	return verdictRank(verdict) >= verdictRank(min)
}

// sortOpportunities ranks Buy-first by short-term verdict, then short-term
// score desc, then ticker asc (stable, deterministic tie-break).
func sortOpportunities(o []opportunityResponse) {
	sort.SliceStable(o, func(i, j int) bool {
		if ri, rj := verdictRank(o[i].ShortTermRule), verdictRank(o[j].ShortTermRule); ri != rj {
			return ri > rj
		}
		if o[i].ShortTermScore != o[j].ShortTermScore {
			return o[i].ShortTermScore > o[j].ShortTermScore
		}
		return o[i].Ticker < o[j].Ticker
	})
}

// ---- Response DTOs (extend docs/api-contract.md §8 with sector/breakdown) ----

type opportunityResponse struct {
	Ticker          string         `json:"ticker"`
	CompanyName     string         `json:"company_name"`
	IndexMembership string         `json:"index_membership"`
	Sector          string         `json:"sector"`
	LastPrice       float64        `json:"last_price"`
	ShortTermRule   string         `json:"short_term_rule"`
	ShortTermScore  int            `json:"short_term_score"`
	LongTermRule    string         `json:"long_term_rule"`
	LongTermScore   int            `json:"long_term_score"`
	ROE             float64        `json:"roe"`
	PER             float64        `json:"per"`
	RiskFlags       []string       `json:"risk_flags"`
	ShortBreakdown  map[string]int `json:"short_term_breakdown"`
	LongBreakdown   map[string]int `json:"long_term_breakdown"`
}

type opportunitiesResponse struct {
	Opportunities []opportunityResponse `json:"opportunities"`
}

type lookupResponse struct {
	Ticker          string `json:"ticker"`
	Name            string `json:"name,omitempty"`
	IndexMembership string `json:"index_membership,omitempty"`
	Sector          string `json:"sector,omitempty"`
	InUniverse      bool   `json:"in_universe"`
	Illiquid        bool   `json:"illiquid"` // true when not in LQ45/Kompas100
}

func companyName(prefer, fallback string) string {
	if prefer != "" {
		return prefer
	}
	return fallback
}

// buildOpportunities returns the ranked, filtered, non-held scored universe.
// Tickers without stored market data + both-horizon scores are omitted (they
// can't be ranked); the refresh endpoint populates them.
func buildOpportunities(db *gorm.DB, filter, minVerdict, q string) ([]opportunityResponse, error) {
	qUp := strings.ToUpper(strings.TrimSpace(q))

	// Held tickers are excluded from the ranked list (issue: non-held universe).
	heldSet := make(map[string]struct{})
	var positions []Position
	if err := db.Find(&positions).Error; err != nil {
		return nil, err
	}
	for _, p := range positions {
		heldSet[p.Ticker] = struct{}{}
	}

	var opps []Opportunity
	if err := db.Order("ticker ASC").Find(&opps).Error; err != nil {
		return nil, err
	}

	// Index market data + analysis for O(1) lookup per universe ticker.
	var mdRows []MarketData
	if err := db.Find(&mdRows).Error; err != nil {
		return nil, err
	}
	market := make(map[string]MarketData, len(mdRows))
	for _, m := range mdRows {
		market[m.Ticker] = m
	}

	type arKey struct{ ticker, horizon string }
	var arRows []AnalysisResult
	if err := db.Find(&arRows).Error; err != nil {
		return nil, err
	}
	ars := make(map[arKey]AnalysisResult, len(arRows))
	for _, ar := range arRows {
		ars[arKey{ar.Ticker, ar.Horizon}] = ar
	}

	out := make([]opportunityResponse, 0, len(opps))
	for _, o := range opps {
		if !inIndex(o.IndexMembership, filter) {
			continue
		}
		if qUp != "" && !strings.Contains(o.Ticker, qUp) && !strings.Contains(strings.ToUpper(o.Name), qUp) {
			continue
		}
		if _, held := heldSet[o.Ticker]; held {
			continue
		}

		md, ok := market[o.Ticker]
		if !ok || md.LastPrice <= 0 {
			continue // unscored — can't rank
		}
		short := ars[arKey{o.Ticker, horizonShort}]
		long := ars[arKey{o.Ticker, horizonLong}]
		if short.ID == "" || long.ID == "" {
			continue // need both-horizon verdicts
		}
		if !meetsMinVerdict(short.RuleVerdict, minVerdict) {
			continue
		}

		out = append(out, opportunityResponse{
			Ticker:          o.Ticker,
			CompanyName:     companyName(md.CompanyName, o.Name),
			IndexMembership: o.IndexMembership,
			Sector:          o.Sector,
			LastPrice:       md.LastPrice,
			ShortTermRule:   short.RuleVerdict,
			ShortTermScore:  short.RuleScore,
			LongTermRule:    long.RuleVerdict,
			LongTermScore:   long.RuleScore,
			ROE:             md.ROE,
			PER:             md.PERatio,
			RiskFlags:       riskFlagsOf(short),
			ShortBreakdown:  toRuleDTO(short).Breakdown,
			LongBreakdown:   toRuleDTO(long).Breakdown,
		})
	}

	sortOpportunities(out)
	return out, nil
}

// ---- HTTP handlers ----

// GET /api/v1/opportunities?filter=lq45&min_verdict=BUY&q=bbc
// Returns Buy-first ranked non-held scored candidates from the liquid universe.
func listOpportunities(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		opps, err := buildOpportunities(db, c.Query("filter"), c.Query("min_verdict"), c.Query("q"))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat peluang"})
		}
		return c.JSON(opportunitiesResponse{Opportunities: opps})
	}
}

// GET /api/v1/opportunities/lookup?q=TICKER
// Custom-ticker search: reports universe membership; out-of-universe → illiquid.
func lookupOpportunity(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ticker := strings.ToUpper(strings.TrimSpace(c.Query("q")))
		if ticker == "" {
			return c.Status(400).JSON(fiber.Map{"error": "q wajib diisi"})
		}
		var opp Opportunity
		inUniverse := db.First(&opp, "ticker = ?", ticker).Error == nil
		resp := lookupResponse{Ticker: ticker, InUniverse: inUniverse, Illiquid: !inUniverse}
		if inUniverse {
			resp.Name = opp.Name
			resp.IndexMembership = opp.IndexMembership
			resp.Sector = opp.Sector
		}
		return c.JSON(resp)
	}
}

// runRefreshOpportunities re-fetches market data + re-scores the whole
// opportunity universe (held tickers are refreshed by the shared market-data
// refresh path, not duplicated here).
func runRefreshOpportunities(db *gorm.DB) (refreshed, failed int, lastUpdate time.Time, err error) {
	var opps []Opportunity
	if err = db.Find(&opps).Error; err != nil {
		return 0, 0, time.Time{}, err
	}
	tickers := make([]string, 0, len(opps))
	for _, o := range opps {
		tickers = append(tickers, o.Ticker)
	}
	// ponytail: refresh is sequential (~1-2s/ticker × N). For a larger universe,
	// add bounded concurrency (e.g. a worker pool) — correct as-is, just slow.
	refreshed, failed, lastUpdate = refreshTickers(db, tickers)
	return refreshed, failed, lastUpdate, nil
}

// POST /api/v1/opportunities/refresh — re-fetch + re-score the universe.
func refreshOpportunities(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		refreshed, failed, lastUpdate, err := runRefreshOpportunities(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal membaca daftar peluang"})
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
