package main

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Position is a held IHSG stock. Stored in SQLite via GORM.
// Market-dependent fields (current price, verdicts) are computed in later
// slices once MarketData/AnalysisResult exist; T1 only persists cost basis.
type Position struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Ticker      string    `gorm:"not null" json:"ticker"`
	Shares      int       `gorm:"not null" json:"shares"`
	AvgBuyPrice float64   `gorm:"not null" json:"avg_buy_price"`
	BuyDate     string    `gorm:"not null" json:"buy_date"` // YYYY-MM-DD
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// --- Request DTOs ---

type createPositionRequest struct {
	Ticker      string  `json:"ticker"`
	Shares      int     `json:"shares"`
	AvgBuyPrice float64 `json:"avg_buy_price"`
	BuyDate     string  `json:"buy_date"`
}

// Pointer fields → partial update (PUT). Only provided fields are changed.
type updatePositionRequest struct {
	Ticker      *string  `json:"ticker"`
	Shares      *int     `json:"shares"`
	AvgBuyPrice *float64 `json:"avg_buy_price"`
	BuyDate     *string  `json:"buy_date"`
}

// --- Response DTOs (match docs/api-contract.md shapes) ---

type summaryResponse struct {
	TotalInvestmentIDR float64  `json:"total_investment_idr"`
	CurrentValueIDR    *float64 `json:"current_value_idr"`     // null until T2 market data
	TotalProfitLossIDR *float64 `json:"total_profit_loss_idr"` // null until T2
	TotalProfitLossPct *float64 `json:"total_profit_loss_pct"` // null until T2
	LastMarketUpdate   *string  `json:"last_market_update"`    // null until T2
}

type verdictSet struct {
	Rule         *string `json:"rule"` // null until T2/T3
	AI           *string `json:"ai"`   // null until T3
	Disagreement bool    `json:"disagreement"`
}

type positionResponse struct {
	ID              string   `json:"id"`
	Ticker          string   `json:"ticker"`
	Shares          int      `json:"shares"`
	AvgBuyPrice     float64  `json:"avg_buy_price"`
	BuyDate         string   `json:"buy_date"`
	CurrentPrice    *float64 `json:"current_price"`     // null until T2
	CurrentValueIDR *float64 `json:"current_value_idr"` // null until T2
	ProfitLossIDR   *float64 `json:"profit_loss_idr"`   // null until T2
	ProfitLossPct   *float64 `json:"profit_loss_pct"`   // null until T2
	WeightPct       float64  `json:"weight_pct"`        // cost-basis allocation, computed
	Verdicts        *struct {
		ShortTerm verdictSet `json:"short_term"`
		LongTerm  verdictSet `json:"long_term"`
	} `json:"verdicts"` // null until T2/T3
}

type portfolioResponse struct {
	Summary   summaryResponse    `json:"summary"`
	Positions []positionResponse `json:"positions"`
}

// validatePositionInput checks the trust-boundary inputs. Returns the
// normalized (uppercased, trimmed) ticker.
func validatePositionInput(ticker string, shares int, avg float64, date string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(ticker))
	if t == "" {
		return "", errors.New("ticker wajib diisi")
	}
	if shares <= 0 {
		return "", errors.New("shares harus lebih dari 0")
	}
	if avg <= 0 {
		return "", errors.New("avg_buy_price harus lebih dari 0")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", errors.New("buy_date harus format YYYY-MM-DD")
	}
	return t, nil
}

// GET /api/v1/portfolio — summary (cost basis + live P&L) + positions with
// allocation %. When MarketData exists for a ticker, current price/value/P&L
// are populated; otherwise those fields stay nil (data not yet refreshed).
func listPortfolio(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var positions []Position
		if err := db.Order("ticker ASC").Find(&positions).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat portofolio"})
		}

		// Index market data by ticker for O(1) lookup per position.
		var mdRows []MarketData
		if err := db.Find(&mdRows).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat data pasar"})
		}
		market := make(map[string]MarketData, len(mdRows))
		var lastUpdate time.Time
		for _, m := range mdRows {
			market[m.Ticker] = m
			if m.UpdatedAt.After(lastUpdate) {
				lastUpdate = m.UpdatedAt
			}
		}

		// Index rule verdicts by ticker+horizon (T3).
		type arKey struct{ ticker, horizon string }
		var arRows []AnalysisResult
		if err := db.Find(&arRows).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat verdict"})
		}
		verdicts := make(map[arKey]AnalysisResult, len(arRows))
		for _, ar := range arRows {
			verdicts[arKey{ar.Ticker, ar.Horizon}] = ar
		}

		totalCost, totalValue := 0.0, 0.0
		hasMarketData := false
		for _, p := range positions {
			totalCost += float64(p.Shares) * p.AvgBuyPrice
			if md, ok := market[p.Ticker]; ok && md.LastPrice > 0 {
				totalValue += float64(p.Shares) * md.LastPrice
				hasMarketData = true
			}
		}
		// Allocation denominator = current value when any market data exists,
		// else cost basis (per docs/api-contract.md weight_pct semantics).
		allocDenom := totalCost
		if hasMarketData && totalValue > 0 {
			allocDenom = totalValue
		}

		items := make([]positionResponse, 0, len(positions))
		for _, p := range positions {
			cost := float64(p.Shares) * p.AvgBuyPrice
			weight := 0.0
			if allocDenom > 0 {
				weight = valueOrCost(p, market) / allocDenom * 100
			}
			pr := positionResponse{
				ID:          p.ID,
				Ticker:      p.Ticker,
				Shares:      p.Shares,
				AvgBuyPrice: p.AvgBuyPrice,
				BuyDate:     p.BuyDate,
				WeightPct:   weight,
			}
			if md, ok := market[p.Ticker]; ok && md.LastPrice > 0 {
				value := float64(p.Shares) * md.LastPrice
				pr.CurrentPrice = ptr(md.LastPrice)
				pr.CurrentValueIDR = ptr(value)
				pl := value - cost
				pr.ProfitLossIDR = ptr(pl)
				if cost > 0 {
					pr.ProfitLossPct = ptr((md.LastPrice/p.AvgBuyPrice - 1) * 100)
				}
			}
			// Populate rule + AI verdicts when an AnalysisResult exists (T3/T4).
			// AI stays nil until a Hermes run stores AIVerdict; disagreement is
			// true only when both sides are present and differ.
			var shortSet, longSet verdictSet
			hasVerdict := false
			if ar, ok := verdicts[arKey{p.Ticker, horizonShort}]; ok && ar.ID != "" {
				rv := ar.RuleVerdict
				shortSet.Rule = &rv
				if ar.AIVerdict != "" {
					av := ar.AIVerdict
					shortSet.AI = &av
					shortSet.Disagreement = disagree(ar.RuleVerdict, ar.AIVerdict)
				}
				hasVerdict = true
			}
			if ar, ok := verdicts[arKey{p.Ticker, horizonLong}]; ok && ar.ID != "" {
				rv := ar.RuleVerdict
				longSet.Rule = &rv
				if ar.AIVerdict != "" {
					av := ar.AIVerdict
					longSet.AI = &av
					longSet.Disagreement = disagree(ar.RuleVerdict, ar.AIVerdict)
				}
				hasVerdict = true
			}
			if hasVerdict {
				pr.Verdicts = &struct {
					ShortTerm verdictSet `json:"short_term"`
					LongTerm  verdictSet `json:"long_term"`
				}{ShortTerm: shortSet, LongTerm: longSet}
			}
			items = append(items, pr)
		}

		resp := portfolioResponse{
			Summary:   summaryResponse{TotalInvestmentIDR: totalCost},
			Positions: items,
		}
		if hasMarketData {
			resp.Summary.CurrentValueIDR = ptr(totalValue)
			resp.Summary.TotalProfitLossIDR = ptr(totalValue - totalCost)
			if totalCost > 0 {
				resp.Summary.TotalProfitLossPct = ptr((totalValue/totalCost - 1) * 100)
			}
			if !lastUpdate.IsZero() {
				s := lastUpdate.UTC().Format(time.RFC3339)
				resp.Summary.LastMarketUpdate = &s
			}
		}
		return c.JSON(resp)
	}
}

// valueOrCost returns the live market value when data exists, else cost basis.
func valueOrCost(p Position, market map[string]MarketData) float64 {
	if md, ok := market[p.Ticker]; ok && md.LastPrice > 0 {
		return float64(p.Shares) * md.LastPrice
	}
	return float64(p.Shares) * p.AvgBuyPrice
}

func ptr[T any](v T) *T { return &v }

// POST /api/v1/portfolio
func createPosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req createPositionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "body tidak valid"})
		}
		ticker, err := validatePositionInput(req.Ticker, req.Shares, req.AvgBuyPrice, req.BuyDate)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		p := Position{
			ID:          uuid.NewString(),
			Ticker:      ticker,
			Shares:      req.Shares,
			AvgBuyPrice: req.AvgBuyPrice,
			BuyDate:     req.BuyDate,
		}
		if err := db.Create(&p).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal menyimpan posisi"})
		}
		return c.Status(201).JSON(p)
	}
}

// PUT /api/v1/portfolio/:id — partial update of editable fields.
func updatePosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var p Position
		if err := db.First(&p, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "posisi tidak ditemukan"})
		}

		var req updatePositionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "body tidak valid"})
		}

		// Merge provided fields, then validate the resulting state.
		if req.Ticker != nil || req.Shares != nil || req.AvgBuyPrice != nil || req.BuyDate != nil {
			ticker := p.Ticker
			if req.Ticker != nil {
				ticker = *req.Ticker
			}
			shares := p.Shares
			if req.Shares != nil {
				shares = *req.Shares
			}
			avg := p.AvgBuyPrice
			if req.AvgBuyPrice != nil {
				avg = *req.AvgBuyPrice
			}
			date := p.BuyDate
			if req.BuyDate != nil {
				date = *req.BuyDate
			}
			norm, err := validatePositionInput(ticker, shares, avg, date)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": err.Error()})
			}
			p.Ticker, p.Shares, p.AvgBuyPrice, p.BuyDate = norm, shares, avg, date
		}

		if err := db.Save(&p).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memperbarui posisi"})
		}
		return c.JSON(p)
	}
}

// DELETE /api/v1/portfolio/:id
func deletePosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		res := db.Delete(&Position{}, "id = ?", id)
		if res.Error != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal menghapus posisi"})
		}
		if res.RowsAffected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "posisi tidak ditemukan"})
		}
		return c.SendStatus(204)
	}
}
